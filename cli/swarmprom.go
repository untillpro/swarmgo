/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	gc "github.com/untillpro/gochips"
)

const (
	swarmpromFolder                = "swarmprom"
	swarmpromComposeFileName       = swarmpromFolder + "/swarmprom.yml"
	alertmanagerFolder             = swarmpromFolder + "/alertmanager"
	alertmanagerTargetConfigPath   = alertmanagerFolder + "/alertmanager.yml"
	alertmanagerSlackConfigPath    = alertmanagerFolder + "/alertmanager-slack.yml"
	alertmanagerNoAlertsConfigPath = alertmanagerFolder + "/alertmanager-noalerts.yml"

	prometheusLabel = "prometheus"
)

var (
	argSlackWebhookURL  string
	argGrafanaPass      string
	argPrometheusPass   string
	argAlertmanagerPass string
	argNoAlerts         bool
	argUpgradeSwarmprom bool
)

type infoForCopy struct {
	host   string
	client *SSHClient
}

var swarmpromCmd = &cobra.Command{
	Use:   "mon",
	Short: "Adds starter kit for swarm monitoring",
	Long:  `Deploys Prometheus, cAdvisor, Node Exporter, Alert Manager and Grafana to the current swarm`,
	Run: loggedCmd(func(cmd *cobra.Command, args []string) {
		checkSSHAgent()
		if argUpgradeSwarmprom {
			UpgradeAlertmanagerCfg(argNoAlerts, argSlackWebhookURL)
		} else {
			DeploySwarmprom(argNoAlerts, argSlackWebhookURL, argGrafanaPass, argPrometheusPass, argAlertmanagerPass)
		}
	}),
}

// DeploySwarmprom deploys swarmprom
func DeploySwarmprom(noalerts bool, slackWebhook string, grafanaPass string, prometheusPass string, alertMgrPass string) {
	gc.Info("Preparing to deploy swarmprom")
	firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
	gc.ExitIfFalse(firstEntry.node.Traefik, "Need to deploy traefik before deploy")
	client := getSSHClient(clusterFile)
	checkSwarmNodeLabelTrue(clusterFile, firstEntry, prometheusLabel, true)

	if len(grafanaPass) == 0 {
		grafanaPass = readPasswordPrompt(fmt.Sprintf("Specify [%s] password (access to Grafana web-ui)", clusterFile.GrafanaUser))
	}
	if len(alertMgrPass) == 0 {
		alertMgrPass = readPasswordPrompt(fmt.Sprintf("Specify [%s] password (access to Alertmanager web-ui)", clusterFile.AlertmanagerUser))
	}
	if len(prometheusPass) == 0 {
		prometheusPass = readPasswordPrompt(fmt.Sprintf("Specify [%s] password (access to Prometheus web-ui)", clusterFile.PrometheusUser))
	}

	clusterFile.GrafanaPassword = grafanaPass

	getSlackWebhookURL(clusterFile, noalerts, slackWebhook)

	host := firstEntry.node.Host
	forCopy := infoForCopy{
		host, client,
	}
	gc.Info("Installing dos2unix")
	client.ExecOrExit(host, "sudo apt-get install dos2unix")
	gc.Info("Installing htpasswd")
	client.ExecOrExit(host, "sudo apt-get install apache2-utils -y")

	clusterFile.PrometheusBasicAuth = client.ExecOrExit(host, fmt.Sprintf("htpasswd -nbB %s \"%s\"", clusterFile.PrometheusUser, prometheusPass))
	clusterFile.PrometheusBasicAuth = strings.ReplaceAll(clusterFile.PrometheusBasicAuth, "$", "\\$\\$")
	clusterFile.AlertManagerBasicAuth = client.ExecOrExit(host, fmt.Sprintf("echo $(htpasswd -nbB %s \"%s\")", clusterFile.AlertmanagerUser, alertMgrPass))
	clusterFile.AlertManagerBasicAuth = strings.ReplaceAll(clusterFile.AlertManagerBasicAuth, "$", "\\$\\$")

	copyToHost(&forCopy, filepath.ToSlash(filepath.Join(getSourcesDir(), swarmpromFolder)))
	templateAndCopy(client, host, swarmpromComposeFileName, "~/"+swarmpromComposeFileName, clusterFile)
	writeAlertManagerConf(client, host, clusterFile, noalerts)

	gc.Info("Deploying swarmprom")
	client.ExecOrExit(host, "sudo docker stack deploy -c "+swarmpromComposeFileName+" prom")
	gc.Info("Swarmprom deployed")

	if !noalerts {
		gc.Info("Testing alerts")
		gc.ExitIfError(postTestMessageToAlertmanager(clusterFile.WebhookURL, clusterFile.ChannelName))
	} else {
		gc.Info("Push alerts disabled")
	}
}

// UpgradeAlertmanagerCfg upgrades alert manager notifications configuration
func UpgradeAlertmanagerCfg(noalerts bool, slackWebhook string) {
	firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
	gc.ExitIfFalse(firstEntry.node.Traefik, "Need to deploy traefik before deploy")
	getSlackWebhookURL(clusterFile, noalerts, slackWebhook)
	client := getSSHClient(clusterFile)
	host := firstEntry.node.Host
	out, err := client.Exec(host, "sudo docker service ps prom_alertmanager")
	gc.ExitIfError(err, "Unable to find deployed alert manager service. Nothing to update")
	gc.ExitIfFalse(strings.Contains(out, "Running"), "Alert manager service doesn't seem to be running")
	forCopy := infoForCopy{
		host, client,
	}
	gc.Info("Installing dos2unix")
	client.ExecOrExit(host, "sudo apt-get install dos2unix")

	copyToHost(&forCopy, filepath.ToSlash(filepath.Join(getSourcesDir(), alertmanagerFolder)))
	writeAlertManagerConf(client, host, clusterFile, noalerts)

	cfgName := "alert_conf_" + time.Now().Format("20060102150405")

	out, err = client.Exec(host, "sudo docker service inspect prom_alertmanager | grep -m 1 alert_conf")
	gc.ExitIfError(err)
	out = strings.Trim(out, " \n\t")
	gc.ExitIfFalse(strings.HasPrefix(out, "\"ConfigName\": \""), "Unexpected output from service inspect: "+out)
	oldCfgName := out[15 : len(out)-1]
	gc.Info("Old configuration name:", oldCfgName, "New configuration name:", cfgName)

	client.ExecOrExit(host, fmt.Sprintf("sudo docker config create %s ~/%s", cfgName, alertmanagerTargetConfigPath))
	client.ExecOrExit(host, fmt.Sprintf("sudo docker service update --config-rm %s --config-add src=%s,target=/etc/alertmanager/alertmanager.yml prom_alertmanager", oldCfgName, cfgName))
	if !noalerts {
		gc.Info("Testing alerts")
		gc.ExitIfError(postTestMessageToAlertmanager(clusterFile.WebhookURL, clusterFile.ChannelName))
	} else {
		gc.Info("Push alerts disabled")
	}
}

func templateAndCopy(client *SSHClient, host, localFile, destFile string, clusterFile *clusterFile) {
	appliedBuffer := executeTemplateToFile(localFile, clusterFile)
	client.ExecOrExit(host, "cat > "+destFile+" << EOF\n\n"+appliedBuffer.String()+"\nEOF")
	gc.Info(destFile, fmt.Sprintf("Copied and applied by template '%s'->'%s'", localFile, destFile))
}

func getSlackWebhookURL(clusterFile *clusterFile, noalerts bool, slackWebhookURL string) {
	if !noalerts {
		if len(slackWebhookURL) == 0 {
			gc.Info("Enter webhook URL for slack channel", clusterFile.ChannelName)
			clusterFile.WebhookURL = waitUserInput()
		} else {
			gc.Info("Setting webhook URL for slack channel to ", slackWebhookURL)
			clusterFile.WebhookURL = slackWebhookURL
		}
	} else {
		clusterFile.WebhookURL = ""
	}
}

func writeAlertManagerConf(client *SSHClient, host string, clusterFile *clusterFile, noalerts bool) {
	alertMgrSrcCfg := alertmanagerSlackConfigPath
	if noalerts {
		alertMgrSrcCfg = alertmanagerNoAlertsConfigPath
	}

	templateAndCopy(client, host, alertMgrSrcCfg, "~/"+alertmanagerTargetConfigPath, clusterFile)
}

func copyToHost(forCopy *infoForCopy, src string) {
	info, err := os.Lstat(src)
	gc.ExitIfError(err)
	if info.IsDir() {
		copyDirToHost(src, forCopy)
	} else {
		copyFileToHost(src, forCopy)
	}
}

func copyDirToHost(dirPath string, forCopy *infoForCopy) {
	forCopy.client.ExecOrExit(forCopy.host, "mkdir -p "+substringAfter(dirPath,
		filepath.ToSlash(getSourcesDir())+"/"))
	dirContent, err := ioutil.ReadDir(dirPath)
	gc.ExitIfError(err)
	for _, dirEntry := range dirContent {
		src := filepath.ToSlash(filepath.Join(dirPath, dirEntry.Name()))
		copyToHost(forCopy, src)
	}
}

func copyFileToHost(filePath string, forCopy *infoForCopy) {
	relativePath := substringAfter(filePath, filepath.ToSlash(getSourcesDir())+"/")
	err := forCopy.client.CopyPath(forCopy.host, filePath, relativePath)
	gc.ExitIfError(err)
	forCopy.client.ExecOrExit(forCopy.host, "sudo dos2unix "+relativePath)
	forCopy.client.ExecOrExit(forCopy.host, "sudo chown root:root "+relativePath)
	forCopy.client.ExecOrExit(forCopy.host, "sudo chmod 777 "+relativePath)
	gc.ExitIfError(err)
	gc.Info(relativePath, "copied on host")
}

func postTestMessageToAlertmanager(URL, channelName string) error {
	jsonMap := map[string]string{"channel": channelName, "username": "alertmanager", "text": "Alertmanager successfully installed to cluster", "icon_emoji": ":ghost:"}
	jsonEntry, _ := json.Marshal(jsonMap)
	_, err := http.Post(URL, "application/json", bytes.NewReader(jsonEntry))
	return err
}

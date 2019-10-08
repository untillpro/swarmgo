/*
 * Copyright (c) 2018-present unTill Pro, Ltd. and Contributors
 *
 * This source code is licensed under the MIT license found in the
 * LICENSE file in the root directory of this source tree.
 *
 */

package swarmgo

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
)

var (
	argSlackWebhookURL  string
	argNoAlerts         bool
	argUpgradeSwarmprom bool
)

type infoForCopy struct {
	host   string
	client *SSHClient
}

var swarmpromCmd = &cobra.Command{
	Use:   "swarmprom",
	Short: "Create starter kit for swarm monitoring",
	Long:  `Deploys Prometheus, WebhookURL, cAdvisor, Node Exporter, Alert Manager and Unsee to the current swarm`,
	Run: loggedCmd(func(args []string) {
		checkSSHAgent()
		firstEntry, clusterFile := getSwarmLeaderNodeAndClusterFile()
		gc.ExitIfFalse(firstEntry.node.Traefik, "Need to deploy traefik before swarmprom deploy")

		if argUpgradeSwarmprom {
			updateAlertMgrCfg(clusterFile, firstEntry)
		} else {
			deploySwarmprom(clusterFile, firstEntry)
		}
	}),
}

func templateAndCopy(client *SSHClient, host, localFile, destFile string, clusterFile *clusterFile) {
	appliedBuffer := executeTemplateToFile(localFile, clusterFile)
	client.ExecOrExit(host, "cat > "+destFile+" << EOF\n\n"+appliedBuffer.String()+"\nEOF")
	gc.Info(destFile, fmt.Sprintf("Copied and applied by template '%s'->'%s'", localFile, destFile))
}

func updateAlertMgrCfg(clusterFile *clusterFile, firstEntry *entry) {
	getSlackWebhookURL(clusterFile)
	client := getSSHClient(clusterFile)
	host := firstEntry.node.Host
	out, err := client.Exec(host, "sudo docker service ps prom_alertmanager")
	gc.ExitIfError(err, "Unable to find deployed alert manager service. Nothing to update")
	gc.ExitIfFalse(strings.Contains(out, "Running"), "Alert manager service doesn't seem to be running")
	forCopy := infoForCopy{
		host, client,
	}
	gc.Info("Trying to install dos2unix")
	client.ExecOrExit(host, "sudo apt-get install dos2unix")

	copyToHost(&forCopy, filepath.ToSlash(filepath.Join(getSourcesDir(), alertmanagerFolder)))
	writeAlertManagerConf(client, host, clusterFile)

	cfgName := "alert_conf_" + time.Now().Format("20060102150405")

	out, err = client.Exec(host, "sudo docker service inspect prom_alertmanager | grep -m 1 alert_conf")
	gc.ExitIfError(err)
	out = strings.Trim(out, " \n\t")
	gc.ExitIfFalse(strings.HasPrefix(out, "\"ConfigName\": \""), "Unexpected output from service inspect: "+out)
	oldCfgName := out[15 : len(out)-1]
	gc.Info("Old configuration name:", oldCfgName, "New configuration name:", cfgName)

	client.ExecOrExit(host, fmt.Sprintf("sudo docker config create %s ~/%s", cfgName, alertmanagerTargetConfigPath))
	client.ExecOrExit(host, fmt.Sprintf("sudo docker service update --config-rm %s --config-add src=%s,target=/etc/alertmanager/alertmanager.yml prom_alertmanager", oldCfgName, cfgName))
	if !argNoAlerts {
		gc.Info("Testing alerts")
		gc.ExitIfError(postTestMessageToAlertmanager(clusterFile.WebhookURL, clusterFile.ChannelName))
	}
}

func getSlackWebhookURL(clusterFile *clusterFile) {
	if !argNoAlerts {
		if len(argSlackWebhookURL) == 0 {
			gc.Info("Enter webhook URL for slack channel", clusterFile.ChannelName)
			clusterFile.WebhookURL = waitUserInput()
		} else {
			gc.Info("Setting webhook URL for slack channel to ", argSlackWebhookURL)
			clusterFile.WebhookURL = argSlackWebhookURL
		}
	} else {
		clusterFile.WebhookURL = ""
	}
}

func writeAlertManagerConf(client *SSHClient, host string, clusterFile *clusterFile) {
	alertMgrSrcCfg := alertmanagerSlackConfigPath
	if argNoAlerts {
		alertMgrSrcCfg = alertmanagerNoAlertsConfigPath
	}

	templateAndCopy(client, host, alertMgrSrcCfg, "~/"+alertmanagerTargetConfigPath, clusterFile)
}

func deploySwarmprom(clusterFile *clusterFile, firstEntry *entry) {
	client := getSSHClient(clusterFile)
	clusterFile.GrafanaPassword = readPasswordPrompt("Grafana admin user password")
	getSlackWebhookURL(clusterFile)
	//TODO don't forget to implement passwords for prometheus and traefik
	host := firstEntry.node.Host
	forCopy := infoForCopy{
		host, client,
	}
	gc.Info("Trying to install dos2unix")
	client.ExecOrExit(host, "sudo apt-get install dos2unix")
	copyToHost(&forCopy, filepath.ToSlash(filepath.Join(getSourcesDir(), swarmpromFolder)))

	templateAndCopy(client, host, swarmpromComposeFileName, "~/"+swarmpromComposeFileName, clusterFile)
	writeAlertManagerConf(client, host, clusterFile)

	gc.Info("Trying to deploy swarmprom")
	client.ExecOrExit(host, "sudo docker stack deploy -c "+swarmpromComposeFileName+" prom")
	gc.Info("Swarmprom deployed")

	if !argNoAlerts {
		gc.Info("Testing alerts")
		gc.ExitIfError(postTestMessageToAlertmanager(clusterFile.WebhookURL, clusterFile.ChannelName))
	}
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

[ -z $1 ] &&  { echo "Use $0 <shapshot>"; exit 1;}

set -e
set -x

VBoxManage controlvm node1 poweroff 2> /dev/null || :
VBoxManage modifyvm node1 --natdnshostresolver1 on

VBoxManage controlvm node2 poweroff 2> /dev/null || :
VBoxManage modifyvm node2 --natdnshostresolver1 on

VBoxManage controlvm node3 poweroff 2> /dev/null || :
VBoxManage modifyvm node3 --natdnshostresolver1 on

VBoxManage snapshot node1 restore $1
VBoxManage startvm node1 --type headless

VBoxManage snapshot node2 restore $1
VBoxManage startvm node2 --type headless

VBoxManage snapshot node3 restore $1
VBoxManage startvm node3 --type headless

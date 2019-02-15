[ -z $1 ] && { echo "Use $0 <shapshot>"; exit 1;}

set -e
set -x

VBoxManage controlvm node1 pause
VBoxManage controlvm node2 pause
VBoxManage controlvm node3 pause
VBoxManage snapshot node1 take $1
VBoxManage snapshot node2 take $1
VBoxManage snapshot node3 take $1
VBoxManage controlvm node1 resume
VBoxManage controlvm node2 resume
VBoxManage controlvm node3 resume
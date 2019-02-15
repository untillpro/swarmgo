([ -z $1 ] || [ -z $2 ]) && { echo "Use $0 <vm> <shapshot>"; exit 1;}
VBoxManage snapshot $1 take $2

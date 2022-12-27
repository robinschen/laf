
## Intro

> WARNING: This is a work in progress. The scripts are not yet ready for production use.

> This script is used to deploy the v1.0 development environment. The v1.0 environment has not been released yet, so this script is only for laf contributors to use in the development environment.

## Create development environment on Linux
 
```bash
cd deploy/scripts

# replace with your domain here. 
export DOMAIN=127.0.0.1.nip.io  

# install k8s cluster
sh install-on-linux.sh $DOMAIN  
```

## Create development environment on MacOS


1. Install multipass on MacOS

```bash
# Skip this step if you have already installed multipass
# see https://multipass.run/install
brew install --cask multipass 
```

2. Create vm & deploy in it 

```bash
cd deploy/scripts
sh install-on-mac.sh  # create vm & setup in it
``` 


----------------multipass config -----------------------------------


1. 修改 multipass 配置
# 设置 multipass 后端为 lxd 或者 virtualbox
sudo multipass set local.driver=lxd
2. 安装lxd
multipass networks
#报错
-------------------------------------------------------------------------
networks failed: Cannot connect to /var/snap/lxd/common/lxd/unix.socket: QLocalSocket::connectToServer: Invalid name

Please ensure the LXD snap is installed and enabled. Also make sure
 the LXD interface is connected via `snap connect multipass:lxd lxd`.
---------------------------------------------------------------------------

#安装 lxd
sudo snap install lxd
multipass networks
-------------------------------------------------------------------------------
Name    Type      Description
enp3s0  ethernet  Ethernet device
mpbr0   bridge    Network bridge for Multipass
--------------------------------------------------------------------------------
# 设置桥接网卡
sudo multipass set local.bridged-network=mpbr0
# 重启，否则会报错
sudo reboot
3. 创建 vm

multipass launch -m 4G -n master --bridged


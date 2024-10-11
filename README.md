# simple-docker
docker 的简易实现。来自《自己动手写docker》。

# 使用
开发环境为 ubuntu 14.06, 内核版本 4.4.0-142-generic。不保证在其他系统下能够正常运行。

首先需要准备一个镜像文件：需要在装有docker的计算机上执行以下命令：
```sh
docker pull busybox
docker run -d busybox top -b
docker export -o busybox.tar <容器ID>
tar -xvf busybox.tar -C busybox/
```
将 busybox.tar 移动到 /root 目录下

编译： `go build .`

运行容器：
```sh
// 创建网络
sudo ./docker network create --driver bridge --subnet 192.168.10.1/24 testbridge
// 运行容器
sudo ./docker run -ti -p 80:80 -net testbridge busybox sh
```

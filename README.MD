1. docker build:
   dk run -p 4000:80 -d homework

2. usage:
   1) curl http://172.17.0.2/version
   2) curl http://172.17.0.2/healthz


#### note for example

**容器网络实验**
namespace 互通

- `ip netns` 是去检查 `/var/run/netns`；但是 docker 却没有将每个容器的 net namespace 暴露在这个目录之下
- 核心理念：
  - 利用 veth-pair，将容器跟 bridge 连接在一起。
  - 所有能够访问这个 bridge 的 net namespace 实际上就相当于连在同一台交换机。所以这些 namespace 就相当于利用了这一台虚拟交换机（bridge）连在一起了



自建网桥，然后 host 跟 net namespace 互 ping


```bash
 +-----------------+--------+                 
 |                 | inft-C | <----- veth-pair
 |  net-namespace  |--------+            |
 |                 |                     |
 +-----------------+                     |
                                         v
                                    +--------+
                                    | intf-B |
                            +-------+--------+-------+
                            |          V-br0         |
                            +------------------------+
                                        ^
                                        |
                            +--------------------------+
                            |  host network namespace  |
                            +--------------------------+
                            | ping -I V-br0 5.5.75.109 |
                            +--------------------------+
                    

#====== step 1: 设置网桥； 
# 首先，我们先增加一个网桥lxcbr0，模仿docker0
brctl addbr V-br0
brctl stp V-br0 off
ifconfig V-br0 5.5.75.1/24 up #为网桥设置IP地址


#===== step 2: 设置 net namespace
# 接下来，我们要创建一个network namespace: net-namespace
# 增加一个 network namesapce 命令为 net-namespace （使用ip netns add命令）
ip netns add net-namespace


#===== step 3: 设置基础 loopback
# 激活 namespace 中的 loopback，
# 即 127.0.0.1（使用ip netns exec net-namespace 来操作 net-namespace 中的命令）
ip netns exec net-namespace   ip link set dev lo up 


#===== step 4: v-eth-pair 一头接入 container；
# 然后，我们需要增加一对虚拟网卡
# 增加一个pair虚拟网卡，注意其中的veth类型(type veth peer)，其中一头要按进容器中
# 一头名为 inftC(v-eth-pair 一头接入 container)，另一头名为 intf-b(v-eth-pair 另一头接入 bridge)
ip link add intf-C type veth peer name intf-B

# 把 intf-C 按到 net-namespace 中，这样容器中就会有一个新的网卡了
ip link set intf-C netns net-namespace

# 把容器里的 intf-C 改名为 eth0 （net namespace 外会冲突，net namespace 内就不会了）
ip netns exec net-namespace  ip link set dev intf-C name eth0 

# 为容器中的网卡分配一个IP地址，并激活它
ip netns exec net-namespace  ifconfig eth0 5.5.75.109/24 up


#===== step 5: v-eth-pair 另一头接入 bridge；然后我们要把 intf-B 添加上网桥上
brctl addif V-br0 intf-B
ifconfig intf-B up

#===== step 6: 配置路由；
# 为容器增加一个路由规则，让容器可以访问外面的网络
ip netns exec net-namespace     ip route add default via 5.5.75.1

# 在/etc/netns下创建network namespce名称为 net-namespace 的目录，
# 然后为这个namespace设置resolv.conf，这样，容器内就可以访问域名了
mkdir -p /etc/netns/net-namespace
echo "nameserver 8.8.8.8" > /etc/netns/net-namespace/resolv.conf


#===== step 7: 检查实验成果
root@cloud-native:/home/aimer# ifconfig 
V-br0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 5.5.75.1  netmask 255.255.255.0  broadcast 5.5.75.255
        inet6 fe80::74a5:79ff:febf:aa98  prefixlen 64  scopeid 0x20<link>
        ether 0e:df:f5:bd:08:6f  txqueuelen 1000  (Ethernet)
        RX packets 13  bytes 824 (824.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 54  bytes 6254 (6.2 KB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

docker0: flags=4099<UP,BROADCAST,MULTICAST>  mtu 1500
        inet 172.17.0.1  netmask 255.255.0.0  broadcast 172.17.255.255
        inet6 fe80::42:c0ff:fefe:6da8  prefixlen 64  scopeid 0x20<link>
        ether 02:42:c0:fe:6d:a8  txqueuelen 0  (Ethernet)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 25  bytes 3754 (3.7 KB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

ens33: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 192.168.8.133  netmask 255.255.255.0  broadcast 192.168.8.255
        inet6 fe80::5033:d8fb:8008:59bc  prefixlen 64  scopeid 0x20<link>
        ether 00:0c:29:de:97:1b  txqueuelen 1000  (Ethernet)
        RX packets 37990  bytes 32590481 (32.5 MB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 12810  bytes 1541531 (1.5 MB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

# 美中不足的是，这个 intf-B 没办法像 docker0 那样隐藏起来
intf-B: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet6 fe80::cdf:f5ff:febd:86f  prefixlen 64  scopeid 0x20<link>
        ether 0e:df:f5:bd:08:6f  txqueuelen 1000  (Ethernet)
        RX packets 13  bytes 1006 (1.0 KB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 73  bytes 8781 (8.7 KB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

lo: flags=73<UP,LOOPBACK,RUNNING>  mtu 65536
        inet 127.0.0.1  netmask 255.0.0.0
        inet6 ::1  prefixlen 128  scopeid 0x10<host>
        loop  txqueuelen 1000  (Local Loopback)
        RX packets 16388  bytes 1758100 (1.7 MB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 16388  bytes 1758100 (1.7 MB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

root@cloud-native:/home/aimer# ping -I V-br0 5.5.75.109
PING 5.5.75.109 (5.5.75.109) from 5.5.75.1 V-br0: 56(84) bytes of data.
64 bytes from 5.5.75.109: icmp_seq=1 ttl=64 time=0.042 ms
64 bytes from 5.5.75.109: icmp_seq=2 ttl=64 time=0.054 ms
^C
--- 5.5.75.109 ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1002ms
rtt min/avg/max/mdev = 0.042/0.048/0.054/0.006 ms
root@cloud-native:/home/aimer# 
```





**手动配置容器网络**

- none 就只有 namespace
- 之所以要映射出来，是为了让 ip netns 的命令可以用
- 自己向 docker 拉起来的容器，把 namespace 跟 docker0 通过 veth-pair 连在一起

```bash
#===== step 1: 创建相应的 net namespace
# 这个目录是定死的，因为 ip netns list 只会检查这里
[root@Jarvis ~]# mkdir -p /var/run/netns
[root@Jarvis ~]# find -L /var/run/netns -type l -delete # 删除所有旧配置
[root@Jarvis ~]# ip netns list
my-net-ns
[root@Jarvis ~]# 

#===== step 2: 采用 NULL 模式启动容器
[root@Jarvis ~]# docker run --network=none  -d nginx
d8fb1a3f38b402fbcfaedbc22fe834303626d129e0bc4408e9dd2906eda2bf14
[root@Jarvis ~]# docker ps
CONTAINER ID   IMAGE     COMMAND                  CREATED         STATUS         PORTS     NAMES
d8fb1a3f38b4   nginx     "/docker-entrypoint.…"   5 seconds ago   Up 4 seconds             compassionate_chaplygin
[root@Jarvis ~]# docker inspect d8fb1a3f38b4 | grep Pid
            "Pid": 4353,
            "PidMode": "",
            "PidsLimit": null,
[root@Jarvis ~]# ifconfig 
docker0: flags=4099<UP,BROADCAST,MULTICAST>  mtu 1500
        inet 172.17.0.1  netmask 255.255.0.0  broadcast 172.17.255.255
        ether 02:42:5d:6f:08:5d  txqueuelen 0  (Ethernet)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0


#===== step 3: 进入容器内部查看网络状况（只有默认的 loopback）
[root@Jarvis ~]# nsenter -t 4353 -n ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
[root@Jarvis ~]# 
# 容器并没有接入这里
[root@Jarvis ~]# brctl show
bridge name	bridge id		STP enabled	interfaces
docker0		8000.02425d6f085d	no		
[root@Jarvis ~]# 

#===== step 4: 把容器的 network namespace 映射出来
[root@Jarvis ~]# ln -s /proc/4353/ns/net /var/run/netns/my-net-ns
[root@Jarvis ~]# ip netns list
my-net-ns
[root@Jarvis ~]# ll  /var/run/netns/my-net-ns
lrwxrwxrwx 1 root root 17 Oct 13 11:05 /var/run/netns/my-net-ns -> /proc/4353/ns/net
[root@Jarvis ~]# 

#===== step 5: 创建 veth-pair 并连接容器 namespace 跟 docker0
[root@Jarvis ~]# ip link add intf-C type veth peer name intf-B
[root@Jarvis ~]# ip link
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
.......
3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN mode DEFAULT group default 
    link/ether 02:42:5d:6f:08:5d brd ff:ff:ff:ff:ff:ff
6: intf-B@intf-C: <BROADCAST,MULTICAST,M-DOWN> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether 5a:cd:60:60:13:da brd ff:ff:ff:ff:ff:ff
7: intf-C@intf-B: <BROADCAST,MULTICAST,M-DOWN> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether ba:c4:a9:88:d5:45 brd ff:ff:ff:ff:ff:ff
[root@Jarvis ~]# brctl addif docker0 intf-B
[root@Jarvis ~]# ip link set intf-B up
[root@Jarvis ~]# ifconfig 
docker0: flags=4099<UP,BROADCAST,MULTICAST>  mtu 1500
        inet 172.17.0.1  netmask 255.255.0.0  broadcast 172.17.255.255
        ether 02:42:5d:6f:08:5d  txqueuelen 0  (Ethernet)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
.......
intf-B: flags=4099<UP,BROADCAST,MULTICAST>  mtu 1500
        ether 5a:cd:60:60:13:da  txqueuelen 1000  (Ethernet)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
......
[root@Jarvis ~]# brctl show
bridge name	bridge id		STP enabled	interfaces
docker0		8000.02425d6f085d	no		intf-B
[root@Jarvis ~]# 


#===== step 6: 连接并配置 intf-C 跟 namespace
[root@Jarvis ~]# ip link set intf-C netns my-net-ns
[root@Jarvis ~]# ip link # 在 host 机看，intf-C 已经看不到了
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP mode DEFAULT group default qlen 1000
    link/ether 00:16:3e:14:8b:fb brd ff:ff:ff:ff:ff:ff
3: docker0: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue state DOWN mode DEFAULT group default 
    link/ether 02:42:5d:6f:08:5d brd ff:ff:ff:ff:ff:ff
6: intf-B@if7: <NO-CARRIER,BROADCAST,MULTICAST,UP> mtu 1500 qdisc noqueue master docker0 state LOWERLAYERDOWN mode DEFAULT group default qlen 1000
    link/ether 5a:cd:60:60:13:da brd ff:ff:ff:ff:ff:ff link-netns my-net-ns
[root@Jarvis ~]# ip netns exec my-net-ns ip link # 在 container 看，intf-C 接进来了
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
7: intf-C@if6: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether ba:c4:a9:88:d5:45 brd ff:ff:ff:ff:ff:ff link-netnsid 0
[root@Jarvis ~]# 
# 你可以留意一下这两个名字 「 7: intf-C@if6 」 跟 「 6: intf-B@if7 」，这不就暗示了连接关系了吗
# 改个接口名字，看起来好一些
[root@Jarvis ~]# ip netns exec my-net-ns ip link
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
7: intf-C@if6: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether ba:c4:a9:88:d5:45 brd ff:ff:ff:ff:ff:ff link-netnsid 0
[root@Jarvis ~]# ip netns exec my-net-ns ip link set dev intf-C name eth0
[root@Jarvis ~]# ip netns exec my-net-ns ip link
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
7: eth0@if6: <BROADCAST,MULTICAST> mtu 1500 qdisc noop state DOWN mode DEFAULT group default qlen 1000
    link/ether ba:c4:a9:88:d5:45 brd ff:ff:ff:ff:ff:ff link-netnsid 0
[root@Jarvis ~]# 
[root@Jarvis ~]# ip netns exec my-net-ns ip link
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN mode DEFAULT group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
7: eth0@if6: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP mode DEFAULT group default qlen 1000
    link/ether ba:c4:a9:88:d5:45 brd ff:ff:ff:ff:ff:ff link-netnsid 0
[root@Jarvis ~]# ip netns exec my-net-ns ifconfig
eth0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        ether ba:c4:a9:88:d5:45  txqueuelen 1000  (Ethernet)
        RX packets 11  bytes 962 (962.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

lo: flags=73<UP,LOOPBACK,RUNNING>  mtu 65536
        inet 127.0.0.1  netmask 255.0.0.0
        loop  txqueuelen 1000  (Local Loopback)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

[root@Jarvis ~]# 
# 具体的网络，跟连接的 bridge 相应（这里用的是 docker0，所以要跟 docker0 匹配）
[root@Jarvis ~]# ifconfig docker0
docker0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 172.17.0.1  netmask 255.255.0.0  broadcast 172.17.255.255
        inet6 fe80::42:5dff:fe6f:85d  prefixlen 64  scopeid 0x20<link>
        ether 02:42:5d:6f:08:5d  txqueuelen 0  (Ethernet)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 5  bytes 446 (446.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

[root@Jarvis ~]# 
[root@Jarvis ~]# ip netns exec my-net-ns ip addr add 172.17.0.110/16 dev eth0
[root@Jarvis ~]# ip netns exec my-net-ns ip route add default via 172.17.0.1
[root@Jarvis ~]# ip netns exec my-net-ns ip addr
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default qlen 1000
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
7: eth0@if6: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default qlen 1000
    link/ether ba:c4:a9:88:d5:45 brd ff:ff:ff:ff:ff:ff link-netnsid 0
    inet 172.17.0.110/16 scope global eth0
       valid_lft forever preferred_lft forever
[root@Jarvis ~]# ip netns exec my-net-ns ifconfig
eth0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 172.17.0.110  netmask 255.255.0.0  broadcast 0.0.0.0
        ether ba:c4:a9:88:d5:45  txqueuelen 1000  (Ethernet)
        RX packets 16  bytes 1312 (1.2 KiB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

lo: flags=73<UP,LOOPBACK,RUNNING>  mtu 65536
        inet 127.0.0.1  netmask 255.0.0.0
        loop  txqueuelen 1000  (Local Loopback)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 0  bytes 0 (0.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0

[root@Jarvis ~]# ip netns exec my-net-ns ip route
default via 172.17.0.1 dev eth0 
172.17.0.0/16 dev eth0 proto kernel scope link src 172.17.0.110 
[root@Jarvis ~]# 

#===== step 7: 检查效果
[root@Jarvis ~]# curl 172.17.0.110
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
[root@Jarvis ~]# 


#===== 对比 docker 自己拉起来的网络
[root@Jarvis ~]# docker inspect 1a830f90e071 | grep IPAddress
            "SecondaryIPAddresses": null,
            "IPAddress": "172.17.0.2",
                    "IPAddress": "172.17.0.2",
[root@Jarvis ~]# curl 172.17.0.2
<!DOCTYPE html>
<html>
<head>
<title>Welcome to nginx!</title>
<style>
html { color-scheme: light dark; }
body { width: 35em; margin: 0 auto;
font-family: Tahoma, Verdana, Arial, sans-serif; }
</style>
</head>
<body>
<h1>Welcome to nginx!</h1>
<p>If you see this page, the nginx web server is successfully installed and
working. Further configuration is required.</p>

<p>For online documentation and support please refer to
<a href="http://nginx.org/">nginx.org</a>.<br/>
Commercial support is available at
<a href="http://nginx.com/">nginx.com</a>.</p>

<p><em>Thank you for using nginx.</em></p>
</body>
</html>
[root@Jarvis ~]# 
[root@Jarvis ~]# dk run -d nginx
1a830f90e0711b8e970331c44af3dfc3157747425736baa18159891e7093a27f
[root@Jarvis ~]# dps
CONTAINER ID   IMAGE     COMMAND                  CREATED         STATUS        PORTS     NAMES
1a830f90e071   nginx     "/docker-entrypoint.…"   3 seconds ago   Up 1 second   80/tcp    sweet_engelbart
[root@Jarvis ~]# ifconfig 
docker0: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet 172.17.0.1  netmask 255.255.0.0  broadcast 172.17.255.255
        inet6 fe80::42:5dff:fe6f:85d  prefixlen 64  scopeid 0x20<link>
        ether 02:42:5d:6f:08:5d  txqueuelen 0  (Ethernet)
        RX packets 7  bytes 1177 (1.1 KiB)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 16  bytes 1256 (1.2 KiB)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
......
# 也会多这么一个接口
veth2bb4dcc: flags=4163<UP,BROADCAST,RUNNING,MULTICAST>  mtu 1500
        inet6 fe80::386f:5bff:fe7a:6fed  prefixlen 64  scopeid 0x20<link>
        ether 3a:6f:5b:7a:6f:ed  txqueuelen 0  (Ethernet)
        RX packets 0  bytes 0 (0.0 B)
        RX errors 0  dropped 0  overruns 0  frame 0
        TX packets 11  bytes 906 (906.0 B)
        TX errors 0  dropped 0 overruns 0  carrier 0  collisions 0
```


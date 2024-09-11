<p align="center">
    <br> <a href="README.md">English</a> | 中文
</p>

![image](logo/logo-horizontal.png)
## ETCD Keeper-v3
这是原始 etcdkeeper 项目的一个分支。原始项目可以在[这里](https://github.com/evildecay/etcdkeeper)找到。与原始版本相比，该项目进行了以下更改：
* 移除了对 etcd v2 的支持，仅保留对 etcd v3 的支持。
* 移除了首次登录必须是root用户的限制。
* 修改了键列表的显示，新增非"/"前缀的键展示。
* 增强了对 YAML 格式的支持，包括 YAML 正确性验证和格式化，支持yaml跟json互转。
* 改进了 etcd 客户端的重用和回收机制，减少了 etcdkeeper 和 etcd 之间的连接数量。
* 配置项通过配置文件进行管理，启动参数仅支持 `-c` 参数来指定配置文件路径。如果默认配置文件(./config.yaml)不存在，将使用默认配置。
* 可以配置多个 etcd 地址，并且通过下拉框选择当前的 etcd 地址。也支持直接编辑 etcd 地址进行连接。

## 安装

### 先决条件
- Go 1.22 或更高版本
- etcd v3

### 从源码构建
1. 克隆仓库：
    ```sh
    git clone https://github.com/welllog/etcdkeeper-v3.git
    cd etcdkeeper-v3
    ```

2. 构建项目：
    ```sh
    go build -o etcdkeeper-v3
    ```

3. 运行项目：
    ```sh
    ./etcdkeeper-v3 [-c /somepath/config.yaml]
    ```

### Docker
1. 克隆仓库：
    ```sh
    git clone https://github.com/welllog/etcdkeeper-v3.git
    cd etcdkeeper-v3
    ```
2. 构建 Docker 镜像：
    ```sh
    docker build -t etcdkeeper-v3 .
    ```
3. 运行 Docker 容器：
    ```sh
    docker run -d -p 8010:8010 -v somepath:/cmd/etc etcdkeeper-v3
    ```

## 配置
默认的配置文件是 `config.yaml`，可以使用 `-c` 参数指定配置文件路径。以下是一个示例配置文件：

```yaml
# etcdkeeper-v3 监听主机
host: 0.0.0.0
# etcdkeeper-v3 监听端口
port: 8010
# 日志级别：debug, info, warn, error, fatal
loglevel:
etcds:
  # 第一个默认
    # etcd 地址
  - endpoints: 127.0.0.1:2379
    # etcd 名称
    name: default
    # 键分隔符
    separator: /
    # tls 配置
    tls:
      enable: false
      certFile:
      keyFile:
      trustedCAFile:
  - endpoints: 127.0.0.1:23179
    name: backup
```

## 截图
![image](etcdkeeper-v3.webp)

## 许可证
该项目使用 MIT 许可证。详情请参阅 [LICENSE](LICENSE) 文件。

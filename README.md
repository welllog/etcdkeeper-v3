<p align="center">
    <br> English | <a href="README-CN.md">中文</a>
</p>

![image](logo/logo-horizontal.png)
## ETCD Keeper-v3
This is a fork of the original etcdkeeper project. The original project can be found [here](https://github.com/evildecay/etcdkeeper). Compared to the original version, this project has made the following changes:
* Removed support for etcd v2, retaining only support for etcd v3.
* Removed the restriction that the first login must be as the root user.
* Modified the key list display to include keys without the "/" prefix.
* Enhanced support for YAML format, including validation of YAML correctness and formatting, and support for conversion between yaml and json.
* Improved the reuse and recycling mechanism of the etcd client, reducing the number of connections between etcdkeeper and etcd.
* Configuration items are managed through a configuration file, and the startup parameter only supports the `-c` parameter to specify the configuration file path. If the default configuration file (./config.yaml) does not exist, the default configuration will be used.
* Multiple etcd addresses can be configured, and the current etcd address can be selected from a drop-down list. Directly editing the etcd address for connection is also supported.
* Allows viewing the historical versions of keys in etcd and comparing them.

## Installation

### Prerequisites
- Go 1.22 or higher
- etcd v3

### Build from source
1. Clone the repository:
    ```sh
    git clone https://github.com/welllog/etcdkeeper-v3.git
    cd etcdkeeper-v3
    ```

2. Build the project:
    ```sh
    go build -o etcdkeeper-v3
    ```

3. Run the project:
    ```sh
    ./etcdkeeper-v3 [-c /somepath/config.yaml]
    ```

### Docker
1. Clone the repository:
    ```sh
    git clone https://github.com/welllog/etcdkeeper-v3.git
    cd etcdkeeper-v3
    ```
2. Build the Docker image:
    ```sh
    docker build -t etcdkeeper-v3 .
3. Run the Docker container:
    ```sh
    docker run -d -p 8010:8010 -v somepath:/cmd/etc etcdkeeper-v3

## Configuration
The default configuration file is `config.yaml`, and the `-c` parameter can be used to specify the configuration file path. Here is an example configuration file:

```yaml
# etcdkeeper-v3 listen host
host: 0.0.0.0
# etcdkeeper-v3 listen port
port: 8010
# log level: debug, info, warn, error, fatal
loglevel:
etcds:
  # first default
    # etcd address
  - endpoints: 127.0.0.1:2379
    # etcd name
    name: default
    # key separator
    separator: /
    # tls config
    tls:
      enable: false
      certFile:
      keyFile:
      trustedCAFile:
  - endpoints: 127.0.0.1:23179
    name: backup
```

## Screenshots
![image](etcdkeeper-v3.webp)

## License
This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

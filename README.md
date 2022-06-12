# ovpn-radius | OpenVPN Radius Plugin

Go-based OpenVPN plugin with Radius Authentication and Accounting support, wrapping [FreeRadius RadClient](https://wiki.freeradius.org/config/Radclient)

## Radius Authentication and Accounting Diagram

![diagram](/radius-diagram.png)

## How to Install

Currently tested on Ubuntu 20.04 and 22.04

```bash
# Install prerequisites
apt install golang, git, freeradius-utils, sqlite3

# Clone repository
git clone https://github.com/rakasatria/ovpn-radius

# Build the project
cd ovpn-radius/src && go build

# Create plugin folder and copy binary file
mkdir -p /etc/openvpn/plugin
cp config.json /etc/openvpn/plugin
cp ovpn-radius /etc/openvpn/plugin

# Create database folder sqlite
mkdir -p /etc/openvpn/plugin/db
touch /etc/openvpn/plugin/db/ovpn-radius.db
chmod -R 777 /etc/openvpn/plugin/db

# Create log file
touch /var/log/openvpn/radius-plugin.log # you can change in config file
chown nobody:nogroup /var/log/openvpn/radius-plugin.log
```

adjust configuration in `config.json` with your own enviroment and data

```json
{
    "LogFile": "/home/rsatria/radius/ovpn-radius/radius-plugin.log",
    "ServerInfo":
    {
      "Identifier": "OpenVPN",
      "IpAddress": "10.10.10.123",
      "PortType": "5",
      "ServiceType": "5"
    },
    "Radius":
    {
      "Authentication":
      {
        "Server": "10.10.10.124:1812",
        "Secret": "s3cr3t"
      },
      "Accounting":
      {
        "Server": "10.10.10.124:1813",
        "Secret": "s3cr3t"
      }
    }
}
```

add additional configuration to `/etc/openvpn/server/server.conf`

```bash
port 1194
proto udp
dev tun
auth-user-pass-verify "/etc/openvpn/plugin/ovpn-radius auth " via-file # authenticate to radius
client-connect "/etc/openvpn/plugin/ovpn-radius start " # sent acounting request start and update to radius
client-disconnect "/etc/openvpn/plugin/ovpn-radius stop " # sent acounting request stop to radius
ca easy-rsa/pki/ca.crt
cert easy-rsa/pki/issued/server.crt
key easy-rsa/pki/private/server.key
dh easy-rsa/pki/dh.pem
server 10.8.0.0 255.255.255.0
ifconfig-pool-persist ipp.txt
push "redirect-gateway def1 bypass-dhcp"
push "dhcp-option DNS 8.8.8.8"
push "dhcp-option DNS 8.8.4.4"
script-security 2 # allow running external script/excute program
keepalive 10 120
persist-key
persist-tun
status openvpn-status.log
```

add aditional configuration to `client.ovpn`

```bash
client
dev tun
proto udp
remote vpn.youdomain.com 1194
resolv-retry infinite
nobind
persist-key
persist-tun
remote-cert-tls server
auth SHA512
auth-user-pass #add this for using user authentication
cipher AES-256-CBC
ignore-unknown-option block-outside-dns
block-outside-dns
verb 3
<ca>
-----BEGIN CERTIFICATE-----
MI...
</ca>
<cert>
-----BEGIN CERTIFICATE-----
MI...
</cert>
<key>
-----BEGIN PRIVATE KEY-----
MI...
-----END PRIVATE KEY-----
</key>
<tls-crypt>
-----BEGIN OpenVPN Static key V1-----
2f01...
-----END OpenVPN Static key V1-----
</tls-crypt>

```

## Hope this would help you! thank you

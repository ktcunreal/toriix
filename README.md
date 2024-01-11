# Toriix
[Torii](https://github.com/ktcunreal/torii) combined with modified version of smux

A simple, easy-to-use tunnel utility written in go.

## Feature
- Connection Multiplexing
- AEAD Cipher
- Lightweight

## Download
Download binary from [github release page](https://github.com/ktcunreal/toriix/releases)

## Build from source
*Tested on Linux / Windows x86_64, Go 1.13.3 or newer*

```
git clone github.com/ktcunreal/toriix
``` 
Install Dependencies:
```
go get golang.org/x/crypto/nacl/secretbox 
```

Build binaries:
```
go build -o toriix *.go 
```

## Usage
You must synchronize the clock on both server and client machines.

### Server

`./toriix -m server -i "0.0.0.0:2222" -e "127.0.0.1:8123" -p "some-long-password"`

or 

`./toriix -c /path/to/config.json`

```
{
    "mode": "server",
    "ingress": "0.0.0.0:2222",
    "egress": "127.0.0.1:8123",
    "key": "some-long-password"
}
```

### Client

`./toriix -m client -i "0.0.0.0:1111" -e "127.0.0.1:2222" -p "some-long-password"`

or 

`./toriix -c /path/to/config.json`

```
{
    "mode": "client",
    "ingress": "0.0.0.0:1111",
    "egress": "127.0.0.1:2222",
    "key": "some-long-password"
}
```


*Use a password consist of alphanumeric and symbols, at least 20 digits in length (Recommended)*

## Reference
https://golang.org/x/crypto/nacl/secretbox

https://github.com/xtaci/kcptun

https://gfw.report/

https://gist.github.com/clowwindy/5947691

## License
[GNU General Public License v3.0](https://raw.githubusercontent.com/ktcunreal/toriix/master/LICENSE)

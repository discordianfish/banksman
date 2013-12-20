# banksman

banksman will render a configuration provided by [Collins](http://tumblr.github.io/collins/).

- If the node in question is in the status "Provisioning", it will return a config specified by "IPXE_CONFIG_NAME"
- If the node is unknown, it will return a config booting kernel and initrd given at command line.


## Usage

    Usage of ./banksman:
      -initrd="http://127.0.0.1:8080/static/": path to registration initrd
      -kernel="http://127.0.0.1:8080/static/": path to registration kernel
      -listen="127.0.0.1:8080": adress to listen on
      -password="admin:first": collins password
      -static="static": path will be served at /static
      -uri="http://localhost:9000/api": url to collins api
      -user="blake": collins user


## Quick start

First you need to create a configuration asset in collins. Then you
need to set the IPXE_CONFIG attribute to iPXE configuration:

    collins-shell asset set_attribute --tag=amd64-ubuntu-precise IPXE_CONFIG '#!ipxe 
    dhcp
    echo Starting Ubuntu x64 installer for ${hostname}
    set base-url http://archive.ubuntu.com/ubuntu/dists/lucid/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64
    kernel ${base-url}/linux
    initrd ${base-url}/initrd.gz

*Config based on [this](https://gist.github.com/robinsmidsrod/2214122)*


Now you can assign this configuration asset to any server asset like this:

    collins-shell asset set_attribute IPXE_CONFIG_NAME amd64-ubuntu-precise --tag=ABCDEFG

Now you just need to point the dhcp pxe filename to `http://your-system/${serial}`
and iPXE will boot whatever config collins provides

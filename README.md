# banksman

banksman will render a configuration provided by [Collins](http://tumblr.github.io/collins/).

## Endpoints

### /ipxe/<tag>

If <tag> is in state `Provisioning`, this endpoint looks up a configuration
asset named like <tag>'s `PRIMARY_ROLE` attribute and return that configuration
asset's attribute `IPXE_CONFIG`.

If <tag> is in state `Maintenance`, `Decommissioned` or `Incomplete`, it will
return a ipxe config pointing to `-kernel` and `-initrd`.


### /config/<tag>(/attribute)

This endpoint looks up a configuration asset named like <tag>'s `PRIMARY_ROLE`
attribute and takes that configuration assets's attribute `CONFIG_<attribute>`
as a template.

Following variables are set:

- Nameserver: equals -nameserver flag
- IpAddress
- Netmask
- Gateway
- Asset (the asset object)
- ConfigUrl: url to config (this) endpoint for this tag
- FinalizeUrl: url to finalize endpoint for this tag

You can refer to those variables like this:

    d-i netcfg/get_ipaddress string {{.IpAddress}}

For more complex examples, see http://golang.org/pkg/text/template/

The command line flag `-pool` selects which pool to use. By default, this
expects each asset to have a adress from pool MGMT.

### /static

This endpoint serves static files from directory `static/`.

### /finalize/<tag>

This endpoint change <tag>'s bootdev (back) to disk and sets it's status to
"Provisioned".

## Usage

    Usage of ./banksman:
      -initrd="http://127.0.0.1:8080/static//initrd.gz": path to registration initrd
      -kernel="http://127.0.0.1:8080/static//kernel": path to registration kernel
      -kopts="console=vga console=ttyS1,115200 BOOTIF=${net0/mac}": options to pass to the registration kernel
      -listen="127.0.0.1:8080": adress to listen on
      -nameserver="8.8.8.8 8.8.4.4": space separated list of dns servers to be used in config endpoint
      -password="admin:first": collins password
      -pool="MGMT": use addresses from this pool when rendering config
      -static="static": path will be served at /static
      -uri="http://localhost:9000/api": url to collins api
      -user="blake": collins user

## Quick start

First you need to create provisioning profile setting up new assets with a
primary role, let's assume we call it 'default'. Make sure all your assets
have a address allocated from whatever pool you specified by `-pool`.

Then create a configuration asset called 'default' and set it's `IPXE_CONFIG`
attribute to something like this:

		#!ipxe 
    dhcp
    set url http://archive.ubuntu.com/ubuntu/dists/precise/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64
    kernel ${url}/linux
    initrd ${url}/initrd.gz
    imgargs linux auto=true url=http://${next-server}:8080/config/${uuid}
    boot

The easiest way to upload your configs to collins is by using [collins-shell]():

    collins-shell asset set_attribute IPXE_CONFIG "`cat ipxe.cfg`" --tag=default


You can set the preseed config by setting the `CONFIG` attribute:

    collins-shell asset set_attribute CONFIG "`cat preseed.cfg`" --tag=default


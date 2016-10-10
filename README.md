# banksman

banksman will render a configuration provided by [Collins](http://tumblr.github.io/collins/).

## Endpoints

### /ipxe/\<tag\>

If <tag> is in state `Provisioning`, this endpoint looks up a configuration
asset named like <tag>'s `PRIMARY_ROLE` attribute and return that configuration
asset's attribute `CONFIG_IPXE`.

If <tag> is in state `Maintenance`, `Decommissioned` or `Incomplete`, it will
return a ipxe config pointing to `-kernel` and `-initrd`.


### /config/\<tag\>(/attribute)

This endpoint looks up a configuration asset named like <tag>'s `PRIMARY_ROLE`
attribute and takes that configuration assets's attribute `CONFIG_<attribute>`
as a template.

Following variables are set:

- Asset (the asset object)
- ConfigUrl: url to config endpoint for this tag (this endpoint)
- FinalizeUrl: url to finalize endpoint for this tag

You can refer to those variables like this:

    d-i netcfg/get_hostname string {{index .Asset.Attributes "HOSTNAME"}}

To select a specific element from array, you can iterate over it and render only
wanted elements. To allow for sub string matches, the `suffix` and `prefix`
functions are available:

- `suffix string substring` returns true if `string` ends in `substring`
- `prefix string substring` returns true if `string` starts with `substring`

A common use case is selecting the right asset address to configure the network:

    {{ range .Asset.Addresses }}
      {{ if prefix .Pool "PROD-" }}
    d-i netcfg/get_ipaddress string {{.Address}}
    d-i netcfg/get_netmask string {{.Netmask}}
    d-i netcfg/get_gateway string {{.Gateway}}
    d-i netcfg/get_nameservers string 8.8.8.8 8.8.4.4
    d-i netcfg/confirm_static boolean true
      {{ end }}
    {{ end }}

For more details, see http://golang.org/pkg/text/template/

### /static

This endpoint serves static files from directory `static/`.

### /finalize/\<tag\>

This endpoint change <tag>'s bootdev (back) to disk and sets it's status to
"Provisioned".

## Usage

    Usage of ./banksman:
      -initrd string
            path to registration initrd (default "http://127.0.0.1:8080/static/initrd.gz")
      -ipmiintf string
            IPMI interface (ipmitool -I X) to use when switching bootdev (default "lanplus")
      -ipmitool string
            path to ipmitool (default "ipmitool")
      -kernel string
            path to registration kernel (default "http://127.0.0.1:8080/static/kernel")
      -kopts string
            options to pass to the registration kernel (default "console=tty0 BOOTIF=${netX/mac}")
      -listen string
            adress to listen on (default "127.0.0.1:8080")
      -password string
            collins password (default "admin:first")
      -static string
            path will be served at /static (default "static")
      -uri string
            url to collins api (default "http://localhost:9000/api")
      -user string
            collins user (default "blake")
      -v    Print version and build info

## Quick start

First you need to create provisioning profile setting up new assets with a
primary role, let's assume we call it 'default'. Make sure all your assets
have a address allocated.

Then create a configuration asset called 'default' and set it's `CONFIG_IPXE`
attribute to something like this:

		#!ipxe 
    dhcp
    set url http://archive.ubuntu.com/ubuntu/dists/precise/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64
    kernel ${url}/linux
    initrd ${url}/initrd.gz
    imgargs linux auto=true url=http://${next-server}:8080/config/${uuid}
    boot

The easiest way to upload your configs to collins is by using [collins-shell]():

    collins-shell asset set_attribute CONFIG_IPXE "`cat ipxe.cfg`" --tag=default

You can set the preseed config by setting the `CONFIG` attribute:

    collins-shell asset set_attribute CONFIG "`cat preseed.cfg`" --tag=default


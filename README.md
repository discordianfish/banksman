# banksman

banksman will render a configuration provided by [Collins](http://tumblr.github.io/collins/).

## Endpoints

### /ipxe/<tag>

If <tag> is in state `Provisioning`, this endpoint looks up a configuration
asset named like <tag>'s `PRIMARY_ROLE` attribute and return that configuration
asset's attribute `IPXE_CONFIG`.

If <tag> is in state `Maintenance`, `Decommissioned` or `Incomplete`, it will
return a ipxe config pointing to `-kernel` and `-initrd`.


### /config/<tag>

This endpoint looks up a configuration asset named like <tag>'s `PRIMARY_ROLE`
attribute and returns that configuration assets's attribute `CONFIG`.


### /static

This endpoint serves static files from directory `static/`.


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

Given you have a provisioning profile setting up new nodes with primary role
'Default', you first need to create a configuration asset called 'default'.
Now you can set it's `IPXE_CONFIG` attribute with something like this:

		#!ipxe 
    dhcp
    set url http://archive.ubuntu.com/ubuntu/dists/lucid/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64
    kernel ${url}/linux
    initrd ${url}/initrd.gz
		imgargs linux auto=true url=http://${next-server}:8080/config/${serial}
		boot


You can use the `edit.sh` tool for that:

    ./edit http://blake:admin:first@localhost:9000/api/asset/default IPXE_CONFIG

You can set the preseed config by setting the `CONFIG` attribute:

    ./edit http://blake:admin:first@localhost:9000/api/asset/default CONFIG


# banksman

banksman will render a configuration provided by [Collins](http://tumblr.github.io/collins/)
if the node in question is in the status "Provisioning"

## TODO
- Add registration ipxe config
- Serve initrds
- (maybe) provide preseed for debian and ubuntu based installs

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

package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"text/template"

	"github.com/discordianfish/banksman/version"

	"gopkg.in/tumblr/go-collins.v0/collins"
)

const (
	ipxeRoot           = "/ipxe/"
	configRoot         = "/config/"
	staticRoot         = "/static/"
	finalizeRoot       = "/finalize/"
	configRegistration = `#!ipxe
dhcp
kernel %s %s collins_url=%s collins_user=%s collins_password=%s collins_tag=%s
initrd %s
boot || shell`
)

var (
	client       *collins.Client
	listen       = flag.String("listen", "127.0.0.1:8080", "adress to listen on")
	uri          = flag.String("uri", "http://localhost:9000/api", "url to collins api")
	user         = flag.String("user", "blake", "collins user")
	password     = flag.String("password", "admin:first", "collins password")
	static       = flag.String("static", "static", "path will be served at /static")
	kernel       = flag.String("kernel", "http://"+*listen+staticRoot+"kernel", "path to registration kernel")
	kopts        = flag.String("kopts", "console=tty0 BOOTIF=${netX/mac}", "options to pass to the registration kernel")
	initrd       = flag.String("initrd", "http://"+*listen+staticRoot+"initrd.gz", "path to registration initrd")
	nameservers  = flag.String("nameserver", "8.8.8.8,8.8.4.4", "comma separated list of dns servers to be used in config endpoint")
	pool         = flag.String("pool", "int", "use addresses from this pool when rendering config")
	ipmitool     = flag.String("ipmitool", "ipmitool", "path to ipmitool")
	ipmiIntf     = flag.String("ipmiintf", "lanplus", "IPMI interface (ipmitool -I X) to use when switching bootdev")
	printVersion = flag.Bool("v", false, "Print version and build info")

	registerStates = []string{"Maintenance", "Decommissioned", "Incomplete"}
)

type config struct {
	Nameserver []string
	IPAddress  string
	Netmask    string
	Gateway    string
	Asset      *collins.Asset
	ConfigURL  string
	FinalzeURL string
}

func handleError(w http.ResponseWriter, errStr string, tag string) {
	msg := fmt.Sprintf("[%s]: %s", tag, errStr)
	_, _, err := client.Logs.Create(tag, &collins.LogCreateOpts{Message: msg, Type: "CRITICAL"})
	if err != nil {
		msg = fmt.Sprintf("%s. Couldn't log error: %s", msg, err)
	}
	log.Println(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

func isRegisterState(asset *collins.Asset) bool {
	if asset == nil {
		return true
	}
	for _, status := range registerStates {
		if asset.Metadata.Status == status {
			return true
		}
	}
	return false
}

func isInstallState(asset *collins.Asset) bool {
	return asset.Metadata.Status == "Provisioning"
}

func findPool(addrs []collins.Address) (collins.Address, error) {
	for _, addr := range addrs {
		if strings.ToLower(addr.Pool) == strings.ToLower(*pool) {
			return addr, nil
		}
	}
	return collins.Address{}, fmt.Errorf("Can't find address from pool %s for asset", *pool)
}

func getConfig(asset *collins.Asset) (*collins.Asset, error) {
	name := asset.Attributes["0"]["PRIMARY_ROLE"]
	if name == "" {
		return nil, fmt.Errorf("PRIMARY_ROLE not set")
	}
	c, _, err := client.Assets.Get(name)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("Configuration asset '%s' not found", name)
	}
	return c, nil
}

func ipmi(asset *collins.Asset, commands ...string) error {
	cmdOpts := []string{
		"-H", asset.IPMI.Address,
		"-U", asset.IPMI.Username, "-P", asset.IPMI.Password,
		"-I", *ipmiIntf,
	}
	cmdOpts = append(cmdOpts, commands...)

	log.Printf("exec: %s %v", *ipmitool, cmdOpts)
	cmd := exec.Command(*ipmitool, cmdOpts...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Couldn't execute ipmi command %s: %s",
			strings.Join(commands, " "), output)
	}
	return nil
}

func renderConfig(name, attrName string, w http.ResponseWriter, r *http.Request) {
	asset, _, err := client.Assets.Get(name)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	configAsset, err := getConfig(asset)
	if err != nil {
		handleError(w, fmt.Sprintf("Couldn't get config: %s", err), asset.Metadata.Tag)
		return
	}
	if configAsset.Attributes["0"][attrName] == "" {
		handleError(w, fmt.Sprintf("Couldn't find attribute %s on %s", attrName, configAsset.Metadata.Tag), asset.Metadata.Tag)
		return
	}
	t, err := template.New("config").Parse(configAsset.Attributes["0"][attrName])
	if err != nil {
		handleError(w, err.Error(), asset.Metadata.Tag)
		return
	}

	address, err := findPool(asset.Addresses)
	if err != nil {
		handleError(w, err.Error(), asset.Metadata.Tag)
		return
	}
	conf := &config{
		Nameserver: strings.Split(*nameservers, ","),
		IPAddress:  address.Address,
		Netmask:    address.Netmask,
		Gateway:    address.Gateway,
		Asset:      asset,
		ConfigURL:  fmt.Sprintf("http://%s%s%s", r.Host, configRoot, name),
		FinalzeURL: fmt.Sprintf("http://%s%s%s", r.Host, finalizeRoot, name),
	}
	if err := t.Execute(w, conf); err != nil {
		handleError(w, fmt.Sprintf("Couldn't render template: %s", err), asset.Metadata.Tag)
	}
}

func handleFinalize(w http.ResponseWriter, r *http.Request) {
	log.Printf("< %s", r.URL)
	name := r.URL.Path[len(finalizeRoot):]
	asset, _, err := client.Assets.Get(name)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := ipmi(asset, "chassis", "bootdev", "disk"); err != nil {
		handleError(w, fmt.Sprintf("Couldn't set bootdev: %s", err), asset.Metadata.Tag)
		return
	}
	if _, err := client.Assets.UpdateStatus(asset.Metadata.Tag, &collins.AssetUpdateStatusOpts{Status: "Provisioned", Reason: "Installer finished"}); err != nil {
		handleError(w, fmt.Sprintf("Couldn't set status to Provisioned: %s", err), asset.Metadata.Tag)
		return
	}
	fmt.Fprintf(w, "Successfully finalized %s", name)
	return
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	log.Printf("< %s", r.URL)
	parts := strings.Split(r.URL.Path[len(configRoot):], "/")
	name := parts[0]
	attrName := "CONFIG"
	if len(parts) > 1 {
		attrName = fmt.Sprintf("%s_%s", attrName, strings.ToUpper(parts[1]))
	}
	renderConfig(name, attrName, w, r)
	return
}

func handlePxe(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len(ipxeRoot):]
	log.Printf("< %s", r.URL)
	asset, _, err := client.Assets.Get(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch {
	case isRegisterState(asset):
		fmt.Fprintf(w, fmt.Sprintf(configRegistration, *kernel, *kopts, *uri, *user, *password, name, *initrd))

	case isInstallState(asset):
		configAsset, err := getConfig(asset)
		if err != nil {
			handleError(w, fmt.Sprintf("Couldn't get config: %s", err), asset.Metadata.Tag)
			return
		}
		fmt.Fprintf(w, configAsset.Attributes["0"]["CONFIG_IPXE"])
	default:
		handleError(w, fmt.Sprintf("Status '%s' not supported", asset.Metadata.Status), asset.Metadata.Tag)
	}
}

func main() {
	flag.Parse()
	if *printVersion {
		log.Printf("banksman %s, revision %s from branch %s built by %s on %s", version.Version, version.Revision, version.Branch, version.BuildUser, version.BuildDate)
		os.Exit(0)
	}
	var err error
	client, err = collins.NewClient(*user, *password, *uri)
	if err != nil {
		log.Fatal(err)
	}
	http.HandleFunc(ipxeRoot, handlePxe)
	http.HandleFunc(configRoot, handleConfig)
	http.HandleFunc(finalizeRoot, handleFinalize)
	http.Handle(staticRoot, http.StripPrefix(staticRoot, http.FileServer(http.Dir(*static))))
	log.Printf("banksman %s (rev: %s) on %s", version.Version, version.Revision, *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}

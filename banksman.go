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
	ipxeRoot     = "/ipxe/"
	configRoot   = "/config/"
	finalizeRoot = "/finalize/"
	staticRoot   = "/static/"

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
	ipmitool     = flag.String("ipmitool", "ipmitool", "path to ipmitool")
	ipmiIntf     = flag.String("ipmiintf", "lanplus", "IPMI interface (ipmitool -I X) to use when switching bootdev")
	printVersion = flag.Bool("v", false, "Print version and build info")

	registerStates = []string{"Maintenance", "Decommissioned", "Incomplete"}

	templateFuncs = template.FuncMap{
		"suffix": strings.HasSuffix,
		"prefix": strings.HasPrefix,
	}
)

type config struct {
	ConfigURL   string
	FinalizeURL string
	Asset       *collins.Asset
}

type handlerFunc func(http.ResponseWriter, *http.Request) (string, error)

func errorHandler(f func(http.ResponseWriter, *http.Request) (string, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tag, err := f(w, r)
		if err == nil {
			return
		}
		msg := fmt.Sprintf("[%s]: %s", tag, err.Error())
		_, _, err = client.Logs.Create(tag, &collins.LogCreateOpts{Message: msg, Type: "CRITICAL"})
		if err != nil {
			msg = fmt.Sprintf("%s. Couldn't log error: %s", msg, err)
		}
		log.Println(msg)
		http.Error(w, msg, http.StatusInternalServerError)
	}
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

func handleFinalize(w http.ResponseWriter, r *http.Request) (string, error) {
	log.Printf("< %s", r.URL)
	tag := r.URL.Path[len(finalizeRoot):]
	asset, _, err := client.Assets.Get(tag)
	if err != nil {
		return tag, err
	}
	if err := ipmi(asset, "chassis", "bootdev", "disk"); err != nil {
		return tag, fmt.Errorf("Couldn't set bootdev: %s", err)
	}
	if _, err := client.Assets.UpdateStatus(asset.Metadata.Tag, &collins.AssetUpdateStatusOpts{Status: "Provisioned", Reason: "Installer finished"}); err != nil {
		return tag, fmt.Errorf("Couldn't set status to Provisioned: %s", err)
	}
	fmt.Fprintf(w, "Successfully finalized %s", tag)
	return tag, nil
}

func handleConfig(w http.ResponseWriter, r *http.Request) (string, error) {
	log.Printf("< %s", r.URL)
	parts := strings.Split(r.URL.Path[len(configRoot):], "/")
	tag := parts[0]
	attrName := "CONFIG"
	if len(parts) > 1 {
		attrName = fmt.Sprintf("%s_%s", attrName, strings.ToUpper(parts[1]))
	}

	asset, _, err := client.Assets.Get(tag)
	if err != nil {
		return tag, err
	}
	configAsset, err := getConfig(asset)
	if err != nil {
		return tag, fmt.Errorf("Couldn't get config: %s", err)
	}
	if configAsset.Attributes["0"][attrName] == "" {
		return tag, fmt.Errorf("Couldn't find attribute %s on %s", attrName, configAsset.Metadata.Tag)
	}
	tmpl, err := template.New("config").Funcs(templateFuncs).Parse(configAsset.Attributes["0"][attrName])
	if err != nil {
		return tag, err
	}

	conf := &config{
		Asset:       asset,
		ConfigURL:   fmt.Sprintf("http://%s%s%s", r.Host, configRoot, tag),
		FinalizeURL: fmt.Sprintf("http://%s%s%s", r.Host, finalizeRoot, tag),
	}
	return tag, tmpl.Execute(w, conf)
}

func handlePxe(w http.ResponseWriter, r *http.Request) (string, error) {
	tag := r.URL.Path[len(ipxeRoot):]
	log.Printf("< %s", r.URL)
	asset, _, err := client.Assets.Get(tag)
	if err != nil {
		return "", err
	}

	switch {
	case isRegisterState(asset):
		fmt.Fprintf(w, fmt.Sprintf(configRegistration, *kernel, *kopts, *uri, *user, *password, tag, *initrd))

	case isInstallState(asset):
		configAsset, err := getConfig(asset)
		if err != nil {
			return tag, fmt.Errorf("Couldn't get config: %s", err)
		}
		fmt.Fprintf(w, configAsset.Attributes["0"]["CONFIG_IPXE"])
	}
	return asset.Metadata.Tag, fmt.Errorf("Satus '%s' not supported", asset.Metadata.Status)
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
	for path, handler := range map[string]handlerFunc{
		ipxeRoot:     handlePxe,
		configRoot:   handleConfig,
		finalizeRoot: handleFinalize,
	} {
		http.HandleFunc(path, errorHandler(handler))
	}
	http.Handle(staticRoot, http.StripPrefix(staticRoot, http.FileServer(http.Dir(*static))))

	log.Printf("banksman %s (rev: %s) on %s", version.Version, version.Revision, *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"text/template"
)

const (
	ipxeRoot           = "/ipxe/"
	configRoot         = "/config/"
	staticRoot         = "/static/"
	configRegistration = `#!ipxe
dhcp
kernel %s %s collins_url=%s collins_user=%s collins_password=%s collins_tag=%s
initrd %s
boot || shell`
)

var (
	collins     = &http.Client{}
	listen      = flag.String("listen", "127.0.0.1:8080", "adress to listen on")
	uri         = flag.String("uri", "http://localhost:9000/api", "url to collins api")
	user        = flag.String("user", "blake", "collins user")
	password    = flag.String("password", "admin:first", "collins password")
	static      = flag.String("static", "static", "path will be served at /static")
	kernel      = flag.String("kernel", "http://"+*listen+staticRoot+"/kernel", "path to registration kernel")
	kopts       = flag.String("kopts", "console=vga console=ttyS1,115200 BOOTIF=${net0/mac}", "options to pass to the registration kernel")
	initrd      = flag.String("initrd", "http://"+*listen+staticRoot+"/initrd.gz", "path to registration initrd")
	nameservers = flag.String("nameserver", "8.8.8.8 8.8.4.4", "space separated list of dns servers to be used in config endpoint")
	pool        = flag.String("pool", "MGMT", "use addresses from this pool when rendering config")

	registerStates = []string{"Maintenance", "Decommissioned", "Incomplete"}
)

type config struct {
	Nameserver string
	IpAddress  string
	Netmask    string
	Gateway    string
	Asset      *collinsAsset
}

type collinsAssetState struct {
	ID     int `json:"ID"`
	Status struct {
		Name        string `json:"NAME"`
		Description string `json:"DESCRIPTION"`
	} `json:"STATUS,omitempty"`
	Name        string `json:"NAME"`
	Label       string `json:"LABEL,omitempty"`
	Description string `json:"DESCRIPTION,omitempty"`
}

// incomplete
type collinsAsset struct {
	Status string `json:"status"`
	Data   struct {
		Asset struct {
			ID     int    `json:"ID"`
			Tag    string `json:"TAG"`
			State  collinsAssetState
			Status string `json:"STATUS"`
			Type   string `json:"TYPE"`
		} `json:"ASSET"`
		Attributes map[string]map[string]string `json:"ATTRIBS"`
	} `json:"data"`
}

type collinsAssetAddress struct {
	ID      int    `json:"ID"`
	Pool    string `json:"POOL"`
	Address string `json:"ADDRESS"`
	Netmask string `json:"NETMASK"`
	Gateway string `json:"GATEWAY"`
}

type collinsAssetAddresses struct {
	Status string `json:"status"`
	Data   struct {
		Addresses []collinsAssetAddress
	}
}

func get(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", *uri+path, nil)
	if err != nil {
		return nil, err
	}
	log.Printf("> %s", req.URL)
	req.SetBasicAuth(*user, *password)

	resp, err := collins.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Error %d: %s", resp.StatusCode, body)
	}
	return body, nil
}

func getAddresses(name string) (*collinsAssetAddresses, error) {
	body, err := get("/asset/" + name + "/addresses")
	if err != nil {
		return nil, err
	}
	adresses := &collinsAssetAddresses{}
	return adresses, json.Unmarshal(body, &adresses)
}

func getAsset(name string) (*collinsAsset, error) {
	if name == "" {
		return nil, fmt.Errorf("Name required")
	}
	body, err := get("/asset/" + name)
	if err != nil {
		return nil, err
	}
	asset := &collinsAsset{}
	return asset, json.Unmarshal(body, &asset)
}

func addLog(message, name string) error {
	v := url.Values{}
	v.Set("message", message)
	v.Set("type", "CRITICAL")

	req, err := http.NewRequest("PUT", *uri+"/asset/"+name+"/log?"+v.Encode(), nil)
	if err != nil {
		return err
	}
	log.Printf("> %s", req.URL)
	req.SetBasicAuth(*user, *password)

	resp, err := collins.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Status code %d unexpected", resp.StatusCode)
	}
	return nil
}

func handleError(w http.ResponseWriter, errStr string, name string) {
	msg := fmt.Sprintf("[%s]: %s", name, errStr)
	err := addLog(msg, name)
	if err != nil {
		msg = fmt.Sprintf("%s. Couldn't log error: %s", msg, err)
	}
	log.Println(msg)
	http.Error(w, msg, http.StatusInternalServerError)
}

func isRegisterState(asset *collinsAsset) bool {
	if asset == nil {
		return true
	}
	for _, status := range registerStates {
		if asset.Data.Asset.Status == status {
			return true
		}
	}
	return false
}

func isInstallState(asset *collinsAsset) bool {
	return asset.Data.Asset.Status == "Provisioning"
}

func findPool(addrs *collinsAssetAddresses) (collinsAssetAddress, error) {
	for _, addr := range addrs.Data.Addresses {
		if addr.Pool == *pool {
			return addr, nil
		}
	}
	return collinsAssetAddress{}, fmt.Errorf("Can't find address from pool %s for asset", *pool)
}

func getConfig(asset *collinsAsset) (*collinsAsset, error) {
	name := asset.Data.Attributes["0"]["PRIMARY_ROLE"]
	if name == "" {
		return nil, fmt.Errorf("PRIMARY_ROLE not set")
	}
	c, err := getAsset(name)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, fmt.Errorf("Configuration asset '%s' not found", name)
	}
	return c, nil
}

func handleConfig(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len(configRoot):]
	log.Printf("< %s", r.URL)
	asset, err := getAsset(name)
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	configAsset, err := getConfig(asset)
	if err != nil {
		handleError(w, fmt.Sprintf("Couldn't get config: %s", err), asset.Data.Asset.Tag)
		return
	}
	t, err := template.New("config").Parse(configAsset.Data.Attributes["0"]["CONFIG"])
	if err != nil {
		handleError(w, err.Error(), asset.Data.Asset.Tag)
		return
	}
	addresses, err := getAddresses(name)
	if err != nil {
		handleError(w, err.Error(), asset.Data.Asset.Tag)
		return
	}

	address, err := findPool(addresses)
	if err != nil {
		handleError(w, err.Error(), asset.Data.Asset.Tag)
		return
	}
	conf := &config{
		Nameserver: *nameservers,
		IpAddress:  address.Address,
		Netmask:    address.Netmask,
		Gateway:    address.Gateway,
		Asset:      asset,
	}
	t.Execute(w, conf)
}

func handlePxe(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len(ipxeRoot):]
	log.Printf("< %s", r.URL)
	asset, err := getAsset(name)
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
			handleError(w, fmt.Sprintf("Couldn't get config: %s", err), asset.Data.Asset.Tag)
			return
		}
		fmt.Fprintf(w, configAsset.Data.Attributes["0"]["IPXE_CONFIG"])
	default:
		handleError(w, fmt.Sprintf("Status '%s' not supported", asset.Data.Asset.Status), asset.Data.Asset.Tag)
	}
}

func main() {
	flag.Parse()
	http.HandleFunc(ipxeRoot, handlePxe)
	http.HandleFunc(configRoot, handleConfig)
	http.Handle(staticRoot, http.StripPrefix(staticRoot, http.FileServer(http.Dir(*static))))
	log.Printf("Listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}

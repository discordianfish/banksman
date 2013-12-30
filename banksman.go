package main

// http://en.wikipedia.org/wiki/Banksman

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
)

const (
	ipxeRoot           = "/ipxe/"
	staticRoot         = "/static/"
	configRegistration = `#!ipxe
dhcp
kernel %s collins_url=%s collins_user=%s collins_password=%s collins_serial=%s
initrd %s
boot || shell`
)

var (
	collins  = &http.Client{}
	listen   = flag.String("listen", "127.0.0.1:8080", "adress to listen on")
	uri      = flag.String("uri", "http://localhost:9000/api", "url to collins api")
	user     = flag.String("user", "blake", "collins user")
	password = flag.String("password", "admin:first", "collins password")
	static   = flag.String("static", "static", "path will be served at /static")
	kernel   = flag.String("kernel", "http://"+*listen+staticRoot+"/kernel", "path to registration kernel")
	initrd   = flag.String("initrd", "http://"+*listen+staticRoot+"/initrd.gz", "path to registration initrd")

	registerStates = []string{"Maintenance", "Decommissioned"}
)

type collinsAssetState struct {
	ID          int    `json:"ID"`
	Status      string `json:"STATUS,omitempty"`
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

func getAsset(name string) (*collinsAsset, error) {
	req, err := http.NewRequest("GET", *uri+"/asset/"+name, nil)
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

func handler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len(ipxeRoot):]
	log.Printf("< %s", r.URL)
	asset, err := getAsset(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch {
	case isRegisterState(asset):
		fmt.Fprintf(w, fmt.Sprintf(configRegistration, *kernel, *uri, *user, *password, name, *initrd))

	case isInstallState(asset):
		ipxeConfigName := asset.Data.Attributes["0"]["IPXE_CONFIG_NAME"]
		if ipxeConfigName == "" {
			handleError(w, "Attribute IPXE_CONFIG_NAME missing", asset.Data.Asset.Tag)
			return
		}

		configAsset, err := getAsset(ipxeConfigName)
		if err != nil {
			handleError(w, fmt.Sprintf("Couldn't get configuration asset '%s'", ipxeConfigName), asset.Data.Asset.Tag)
			return
		}
		if configAsset == nil {
			handleError(w, fmt.Sprintf("Couldn't find configuration asset '%s'", ipxeConfigName), asset.Data.Asset.Tag)
			return
		}

		ipxeConfig := configAsset.Data.Attributes["0"]["IPXE_CONFIG"]
		if ipxeConfig == "" {
			handleError(w, "Attribute IPXE_CONFIG missing", ipxeConfigName)
			return
		}

		fmt.Fprintf(w, ipxeConfig)
	default:
		handleError(w, fmt.Sprintf("Status '%s' not supported", asset.Data.Asset.Status), asset.Data.Asset.Tag)
	}
}

func main() {
	flag.Parse()
	http.HandleFunc(ipxeRoot, handler)
	http.Handle(staticRoot, http.StripPrefix(staticRoot, http.FileServer(http.Dir(*static))))
	log.Printf("Listening on %s", *listen)
	log.Fatal(http.ListenAndServe(*listen, nil))
}

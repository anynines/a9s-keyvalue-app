package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"

	"github.com/valkey-io/valkey-go"
)

type ValkeyDetails struct {
	Password string `json:"password"`
	Port     int    `json:"port"`
	Username string `json:"username"`
}

type ValkeyCredentials struct {
	Host          string        `json:"host"`
	CaCertificate *string       `json:"cacrt"`
	Valkey        ValkeyDetails `json:"valkey"`
}

type ServiceInstance struct {
	Credentials ValkeyCredentials `json:"credentials"`
}

type VcapServices map[string][]ServiceInstance

type KeyValue struct {
	Key   string
	Value string
}

// template store
var templates map[string]*template.Template

// fill template store
func initTemplates() {
	if templates == nil {
		templates = make(map[string]*template.Template)
	}
	templates["index"] = template.Must(template.ParseFiles("templates/index.html", "templates/base.html"))
	templates["new"] = template.Must(template.ParseFiles("templates/new.html", "templates/base.html"))
}

func createCredentials() (ValkeyCredentials, error) {
	// Local
	if os.Getenv("VCAP_SERVICES") == "" {
		host := os.Getenv("VALKEY_HOST")
		if len(host) < 1 {
			err := fmt.Errorf("environment variable VALKEY_HOST not set")
			log.Println(err)
			return ValkeyCredentials{}, err
		}

		password := os.Getenv("VALKEY_PASSWORD")
		if len(password) < 1 {
			err := fmt.Errorf("environment variable VALKEY_PASSWORD not set")
			log.Println(err)
			return ValkeyCredentials{}, err
		}

		username := os.Getenv("VALKEY_USERNAME")
		if len(username) < 1 {
			err := fmt.Errorf("environment variable VALKEY_USERNAME not set")
			log.Println(err)
			return ValkeyCredentials{}, err
		}

		portStr := os.Getenv("VALKEY_PORT")
		if len(portStr) < 1 {
			err := fmt.Errorf("environment variable VALKEY_PORT not set")
			log.Println(err)
			return ValkeyCredentials{}, err
		}

		port, err := strconv.Atoi(portStr)
		if err != nil {
			log.Println(err)
			return ValkeyCredentials{}, err
		}

		credentials := ValkeyCredentials{
			Host: host,
			Valkey: ValkeyDetails{
				Password: password,
				Port:     port,
				Username: username,
			},
		}
		return credentials, nil
	}

	// Cloud Foundry
	// no new read of the env var, the reason is the receiver loop
	var vcapServices VcapServices
	err := json.Unmarshal([]byte(os.Getenv("VCAP_SERVICES")), &vcapServices)
	if err != nil {
		log.Println(err)
		return ValkeyCredentials{}, err
	}

	for _, instances := range vcapServices {
		for _, instance := range instances {
			return instance.Credentials, nil
		}
	}

	err = fmt.Errorf("no valid services found in VCAP_SERVICES")
	log.Println(err)
	return ValkeyCredentials{}, err
}

func renderTemplate(w http.ResponseWriter, name string, template string, viewModel interface{}) {
	tmpl := templates[name]
	err := tmpl.ExecuteTemplate(w, template, viewModel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func NewClient() (valkey.Client, error) {
	credentials, err := createCredentials()
	if err != nil {
		return nil, err
	}
	log.Printf("Connection to:\n%v\n", credentials)

	clientOptions := valkey.ClientOption{
		InitAddress: []string{fmt.Sprintf("%v:%v", credentials.Host, credentials.Valkey.Port)},
		Username:    credentials.Valkey.Username,
		Password:    credentials.Valkey.Password,
		SelectDB:    0,
	}

	if credentials.CaCertificate != nil {
		rootCaPool := x509.NewCertPool()
		ok := rootCaPool.AppendCertsFromPEM([]byte(*credentials.CaCertificate))
		if !ok {
			return nil, fmt.Errorf("failed to create root CA pool using `cacrt`")
		}
		clientOptions.TLSConfig = &tls.Config{
			RootCAs:    rootCaPool,
			ServerName: credentials.Host,
		}
	}

	client, err := valkey.NewClient(clientOptions)

	return client, err
}

// create KV pair
func createKeyValue(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	key := r.PostFormValue("key")
	value := r.PostFormValue("value")

	http.Redirect(w, r, "/", http.StatusFound)

	// insert key value into service
	client, err := NewClient()
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}
	defer client.Close()

	ctx := context.Background()
	err = client.Do(ctx, client.B().Set().Key(key).Value(value).Build()).Error()
	if err != nil {
		log.Printf("Failed to set key %v and value %v ; err = %v", key, value, err)
		return
	}
}

func newKeyValue(w http.ResponseWriter, r *http.Request) {
	renderTemplate(w, "new", "base", nil)
}

func renderKeyValues(w http.ResponseWriter, r *http.Request) {
	keyStore := make([]KeyValue, 0)

	credentials, err := createCredentials()
	if err != nil {
		return
	}
	log.Printf("Connection to:\n%v\n", credentials)

	client, err := NewClient()
	if err != nil {
		log.Printf("Failed to create connection: %v", err)
		return
	}
	defer client.Close()

	ctx := context.Background()
	log.Printf("Collecting keys.\n")
	// collect keys
	keys, err := client.Do(ctx, client.B().Keys().Pattern("*").Build()).AsStrSlice()
	if err != nil {
		log.Printf("Failed to fetch keys, err = %v\n", err)
		return
	}
	for _, key := range keys {
		value, err := client.Do(ctx, client.B().Get().Key(key).Build()).ToString()
		if err != nil {
			log.Printf("Failed to fetch value for key %v, err = %v\n", key, err)
		} else {
			keyStore = append(keyStore, KeyValue{Key: key, Value: value})
		}
	}

	renderTemplate(w, "index", "base", keyStore)
}

func main() {
	initTemplates()

	port := "9090"
	if port = os.Getenv("PORT"); len(port) == 0 {
		port = "9090"
	}

	// https://docs.cloudfoundry.org/devguide/deploy-apps/environment-variable.html#-home
	var dir string
	var err error
	appPath := os.Getenv("HOME")

	dir, _ = filepath.Abs(appPath)
	if os.Getenv("VCAP_SERVICES") == "" {
		dir, err = filepath.Abs("/app")
		if err != nil {
			log.Fatal(err)
		}
	}

	// local testing
	if len(os.Getenv("APP_DIR")) > 0 {
		dir, err = filepath.Abs(os.Getenv("APP_DIR"))
		if err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("Public dir: %v\n", dir)

	fs := http.FileServer(http.Dir(path.Join(dir, "public")))
	http.Handle("/public/", http.StripPrefix("/public/", fs))
	http.HandleFunc("/", renderKeyValues)
	http.HandleFunc("/key-values/new", newKeyValue)
	http.HandleFunc("/key-values/create", createKeyValue)

	log.Printf("Listening on :%v\n", port)
	http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
}

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/digitalocean/godo"
	"github.com/kirsle/configdir"
)

// Version number of the program.
const Version = "1.0.0"

// Command line flags.
var (
	configure      bool
	domainOverride string
	forceUpdate    bool
)

// Config describes the JSON schema for the app's config file.
type Config struct {
	AccessToken string      `json:"accessToken"`
	Domain      string      `json:"domain"`
	LastIPv4    string      `json:"ipv4,omitempty"`
	LastIPv6    string      `json:"ipv6,omitempty"`
	TTL         int         `json:"ttl"`
	RecordTypes RecordTypes `json:"recordTypes"`
	LastRun     string      `json:"lastRun"`
}

// RecordTypes is the config attribute for supporting IPv4 vs. IPv6.
type RecordTypes struct {
	A    bool `json:"A"`
	AAAA bool `json:"AAAA"`
}

// Token returns an OAuth2 token.
func (c *Config) Token() (*oauth2.Token, error) {
	token := &oauth2.Token{
		AccessToken: c.AccessToken,
	}
	return token, nil
}

func init() {
	flag.BoolVar(&configure, "config", false, "(Re)configure your Digital Ocean API key.")
	flag.StringVar(&domainOverride, "domain", "", "Use this domain name instead of the one saved with the config.")
	flag.BoolVar(&forceUpdate, "force", false, "Force update the DNS even if the IP addresses haven't changed.")
}

func main() {
	flag.Parse()
	if configure {
		Setup()
	}

	// Load the config file.
	config, err := LoadConfig()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
	}

	// If no access token configured, run setup.
	if config.AccessToken == "" {
		Setup()
	}

	// Print the last run time.
	if config.LastRun != "" {
		fmt.Printf("Last time this program ran was: %s\n", config.LastRun)
	}

	// Collect our IP address(es).
	var (
		ipv4    net.IP
		ipv6    net.IP
		changed = forceUpdate
	)
	if config.RecordTypes.A {
		ipv4, err = GetExternalIP(4)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Found my IPv4 address: %s\n", ipv4)
		if config.LastIPv4 != ipv4.String() {
			changed = true
		}
	}
	if config.RecordTypes.AAAA {
		ipv6, err = GetExternalIP(6)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Found my IPv6 address: %s\n", ipv6)
		if config.LastIPv6 != ipv6.String() {
			changed = true
		}
	}

	// Do the addresses differ from the last seen ones?
	if changed {
		fmt.Println("My IP address has changed from when I last checked!")
		fmt.Println("Updating DO DNS now!")
		UpdateDNS(config, ipv4, ipv6)
	} else {
		fmt.Println("No changes detected in my IP address")
	}

	// Update the stored configuration to, at the very least, refresh the
	// "last run" time.
	config.LastIPv4 = ipv4.String()
	config.LastIPv6 = ipv6.String()
	WriteConfig(config)
}

// UpdateDNS uses the Digital Ocean API to update your DNS records.
func UpdateDNS(config Config, ipv4, ipv6 net.IP) {
	ctx := context.Background()

	// Authenticate with the OAuth2 token.
	oauthClient := oauth2.NewClient(ctx, &config)
	client := godo.NewClient(oauthClient)

	// The domain name to look up in DO DNS.
	domainName := config.Domain
	if domainOverride != "" {
		domainName = domainOverride
	}

	// Get the DNS records. TODO: support domains with more than 50 records.
	records, _, err := client.Domains.Records(ctx, domainName, &godo.ListOptions{
		PerPage: 50,
	})
	if err != nil {
		fmt.Printf("Could not look up DNS for domain %s: doesn't exist in DO?\n", domainName)
		fmt.Printf("Error given from API: %s\n", err)
		os.Exit(1)
	}

	// Find A and AAAA records, and delete them.
	for _, record := range records {
		if record.Type == "A" || record.Type == "AAAA" {
			fmt.Printf("Delete DNS record %s: %s %s\n", record.Type, record.Name, record.Data)
			_, err = client.Domains.DeleteRecord(ctx, domainName, record.ID)
			if err != nil {
				panic(err)
			}
		}
	}

	// Insert new records.
	for _, recordType := range []string{"A", "AAAA"} {
		// Skip record types that we're not updating.
		if (recordType == "A" && !config.RecordTypes.A) || (recordType == "AAAA" && !config.RecordTypes.AAAA) {
			continue
		}

		var dnsValue net.IP
		if recordType == "A" {
			dnsValue = ipv4
		} else {
			dnsValue = ipv6
		}

		for _, subdomain := range []string{"@", "*"} {
			fmt.Printf("Creating %s record: %s %s\n", recordType, subdomain, dnsValue)
			record := &godo.DomainRecordEditRequest{
				Type: recordType,
				Name: subdomain,
				Data: dnsValue.String(),
				TTL:  config.TTL,
			}

			_, _, err = client.Domains.CreateRecord(ctx, domainName, record)
			if err != nil {
				panic(err)
			}
		}
	}
}

// GetExternalIP gets an external IP address.
func GetExternalIP(version int) (result net.IP, err error) {
	url := fmt.Sprintf("https://ipv%d.myexternalip.com/raw", version)

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	content := strings.TrimSpace(string(body))
	result = net.ParseIP(content)

	return
}

// Setup asks for the configuration properties.
func Setup() {
	fmt.Printf("do-dyn-dns v%s\n\n"+
		"I'm going to ask a few questions to configure this app. (To reconfigure\n"+
		"it in the future, run `do-dyn-dns -config`\n\n",
		Version,
	)

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("You'll need to log in to your Digital Ocean control panel and\n" +
		"create a Personal Access Token from the API dashboard, and paste\n" +
		"the token at the prompt below.\n\n",
	)

	accessToken, err := Prompt(reader, "Digital Ocean Access Token:")
	if err != nil {
		panic(err)
	}

	fmt.Print("\n" +
		"Next, you'll need to make sure your domain name is set up in the\n" +
		"DNS network settings in the Digital Ocean dashboard. At the prompt\n" +
		"below, enter the domain name as it appears in the dashboard,\n" +
		"for example: example.com\n\n",
	)

	domain, err := Prompt(reader, "Domain name from your DNS dashboard:")
	if err != nil {
		panic(err)
	}

	ipv4, _ := BoolPrompt(reader, "Support IPv4 (A records)?")
	ipv6, _ := BoolPrompt(reader, "Support IPv6 (AAAA records)?")

	// Allow them to configure the TTL.
	ttlString, err := Prompt(reader, "DNS Record TTL? [1800]")
	if err != nil {
		panic(err)
	}
	if ttlString == "" {
		ttlString = "1800"
	}

	ttl, err := strconv.Atoi(ttlString)
	if err != nil {
		fmt.Printf("The TTL wasn't a valid number. I'll use 1800 by default.")
		ttl = 1800
	}

	config := Config{
		AccessToken: accessToken,
		Domain:      domain,
		TTL:         ttl,
		RecordTypes: RecordTypes{
			A:    ipv4,
			AAAA: ipv6,
		},
	}
	WriteConfig(config)
}

// Prompt asks a question and gets an answer.
func Prompt(reader *bufio.Reader, question string) (result string, err error) {
	for result == "" {
		fmt.Print(question + " ")
		result, err = reader.ReadString('\n')
		if err != nil {
			return result, err
		}

		if result == "" {
			fmt.Println("You must provide an answer to this question.")
		}
	}

	return strings.TrimSpace(result), nil
}

// BoolPrompt asks a yes/no question of the user.
func BoolPrompt(reader *bufio.Reader, question string) (result bool, err error) {
	for {
		answer, err := Prompt(reader, question+" (Yes or No)")
		if err != nil {
			return false, err
		}

		answer = strings.ToLower(answer)
		if strings.HasPrefix(answer, "y") {
			result = true
			break
		} else if strings.HasPrefix(answer, "n") {
			result = false
			break
		}

		fmt.Println("Please answer 'yes' or 'no' (or 'y' or 'n')")
	}

	return result, nil
}

// LoadConfig loads the saved config file.
func LoadConfig() (config Config, err error) {
	configFile := configdir.LocalConfig("do-dyn-dns.json")

	// If no config file, just return an empty config.
	if _, err = os.Stat(configFile); os.IsNotExist(err) {
		return config, nil
	}

	fh, err := os.Open(configFile)
	if err != nil {
		return config, err
	}
	defer fh.Close()

	decoder := json.NewDecoder(fh)
	decoder.Decode(&config)
	return config, nil
}

// WriteConfig saves the config to disk.
func WriteConfig(config Config) error {
	configFile := configdir.LocalConfig("do-dyn-dns.json")

	// Handle nil IP addresses.
	if config.LastIPv4 == "<nil>" {
		config.LastIPv4 = ""
	}
	if config.LastIPv6 == "<nil>" {
		config.LastIPv6 = ""
	}

	// Update the last run time.
	now := time.Now()
	config.LastRun = now.Format("Mon Jan 2 15:04:05 -0700 MST 2006")

	fh, err := os.Create(configFile)
	if err != nil {
		return err
	}
	defer fh.Close()

	encoder := json.NewEncoder(fh)
	encoder.SetIndent("", "\t")
	encoder.Encode(config)

	return nil
}

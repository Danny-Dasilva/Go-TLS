// +build integration

package cycletls_test

import (
	//"fmt"
	"encoding/json"
	"log"
	"testing"

	cycletls "github.com/Danny-Dasilva/CycleTLS/cycletls"
)


var TLS13Results = []CycleTLSOptions{
	{"b32309a26951912be7dba376398abc3b", // HelloChrome_83
		"771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49171-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-27-21,29-23-24,0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/83.0.4103.106 Safari/537.36",
		200},
	{"aa7744226c695c0b2e440419848cf700", // Firefox 92 on macOS (Catalina)
		"771,4865-4867-4866-49195-49199-52393-52392-49196-49200-49162-49161-49171-49172-156-157-47-53-10,0-23-65281-10-11-35-16-5-51-43-13-45-28-21,29-23-24-25-256-257,0",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:92.0) Gecko/20100101 Firefox/92.0",
		200},
}
func TestTLS_13(t *testing.T) {
	client := cycletls.Init()
	for _, options := range TLS13Results {

		response, err := client.Do("https://ja3er.com/json", cycletls.Options{
			Ja3:       options.Ja3,
			UserAgent: options.UserAgent,
		}, "GET")
		if err != nil {
			t.Fatal("Unmarshal Error")
		}

		if response.Response.Status != options.HTTPResponse {
			t.Fatal("Expected Result Not given")
		} else {
			log.Println("ja3er: ", response.Response.Status)
		}
		ja3resp := new(Ja3erResp)

		err = json.Unmarshal([]byte(response.Response.Body), &ja3resp)
		if err != nil {
			t.Fatal("Unmarshal Error")
		}

		if ja3resp.Ja3Hash != options.Ja3Hash {
			t.Fatal("Expected {} Got {} for Ja3Hash", options.Ja3Hash, ja3resp.Ja3Hash)
		}
		if ja3resp.Ja3 != options.Ja3 {
			t.Fatal("Expected {} Got {} for Ja3", options.Ja3, ja3resp.Ja3)
		}
		if ja3resp.UserAgent != options.UserAgent {
			t.Fatal("Expected {} Got {} for UserAgent", options.UserAgent, ja3resp.UserAgent)
		}


		response, err = client.Do("https://tls13.1d.pw", cycletls.Options{
			Ja3:       options.Ja3,
			UserAgent: options.UserAgent,
		}, "GET")
		if response.Response.Status != options.HTTPResponse {
			t.Fatal("tls1.3 Expected {} Got {} for Ja3Hash", options.Ja3Hash, ja3resp.Ja3Hash)
		} else {
			log.Println("tls13: ", response.Response.Status)
		}

	}
}
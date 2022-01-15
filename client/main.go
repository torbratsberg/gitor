package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
)

var client = &http.Client{
	Transport:     nil,
	CheckRedirect: nil,
	Jar:           nil,
}

type ClientConfig struct {
	RemoteServer struct {
		Address string `yaml:"address"`
		Port    string `yaml:"port"`
		Token   string `yaml:"token"`
	}
}

type Repo struct {
	Name     string
	Branches []string
	Remotes  []string
	Tags     []map[string]string
}

type Parameter struct {
	Key   string
	Value string
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getHomeDir() string {
	homeDir, err := os.UserHomeDir()
	check(err)

	return path.Join(homeDir)
}

var configScriptsFilePath string = path.Join(
	getHomeDir(),
	"/.config/gitor/client-config.yml",
)

func getConfig() (cfg *ClientConfig) {
	f, err := os.Open(configScriptsFilePath)
	check(err)
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	check(err)

	f.Close()

	return
}

var gitorConfig ClientConfig = *getConfig()

func printRepoInfo(repo Repo) {
	// Print repo name
	fmt.Println(repo.Name)

	// Print branches
	fmt.Printf("\n  Branches:\n")
	for i := range repo.Branches {
		fmt.Printf("    ")
		fmt.Println(repo.Branches[i])
	}

	// Print remotes
	fmt.Printf("\n  Remotes:\n")
	for i := range repo.Remotes {
		fmt.Printf("    ")
		fmt.Println(repo.Remotes[i])
	}

	// Print tags
	fmt.Printf("\n  Tags:\n")
	for i := range repo.Tags {
		fmt.Printf("    ")
		fmt.Printf(
			"%s: %s\n",
			repo.Tags[i]["hash"],
			repo.Tags[i]["name"],
		)
	}
}

func encodeToken() string {
	return base64.URLEncoding.EncodeToString(
		[]byte(gitorConfig.RemoteServer.Token),
	)
}

func makeUrl(URLPath string, params []Parameter) (url string) {
	url = fmt.Sprintf(
		"http://%s:%s/%s",
		gitorConfig.RemoteServer.Address,
		gitorConfig.RemoteServer.Port,
		URLPath,
	)
	if len(params) > 0 && params != nil {
		url += "?"
		for i := range params {
			url += params[i].Key + "=" + params[i].Value
			url += "&"
		}
	}

	return
}

func requestAndParse(req *http.Request) []byte {
	// Do the request
	res, err := client.Do(req)
	check(err)

	// Check and handle response status code
	if res.StatusCode != 200 {
		switch res.StatusCode {
		case 401:
			fmt.Println("Unauthorized")
			os.Exit(1)
		case 404:
			fmt.Println("Not found")
			os.Exit(1)
		case 500:
			fmt.Println("Internal server error")
			os.Exit(1)
		default:
			fmt.Println("Unknown error")
			os.Exit(1)
		}
	}

	// Read the body
	body, err := ioutil.ReadAll(res.Body)
	check(err)

	res.Body.Close()

	return body
}

func getRepositories(search string) (result []string) {
	params := []Parameter{}
	if search != "" {
		params = []Parameter{{Key: "search", Value: search}}
	}

	url := makeUrl("get_repositories", params)

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", encodeToken())
	check(err)

	body := requestAndParse(req)

	err = json.Unmarshal(body, &result)
	check(err)

	return
}

func getRepository(repoName string) (result Repo) {
	params := []Parameter{{Key: "repoName", Value: repoName}}
	url := makeUrl("get_repository", params)

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", encodeToken())
	check(err)

	body := requestAndParse(req)

	err = json.Unmarshal(body, &result)
	check(err)

	return
}

func newRepository(repoName string) (result Repo) {
	params := []Parameter{{Key: "repoName", Value: repoName}}
	url := makeUrl("new_repository", params)

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", encodeToken())
	check(err)

	body := requestAndParse(req)

	err = json.Unmarshal(body, &result)
	check(err)

	return
}

func deleteRepository(repoName string) (result string) {
	// Check if we really want to delete the repo
	var consent string
	fmt.Printf("Are you sure you want to delete %s? [y/n]\n", repoName)
	fmt.Scanf("%s", &consent)
	if consent != "y" {
		return "Aborted"
	}

	params := []Parameter{{Key: "repoName", Value: repoName}}
	url := makeUrl("delete_repository", params)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", encodeToken())
	check(err)

	body := requestAndParse(req)
	result = string(body)

	err = json.Unmarshal(body, &result)
	check(err)

	return result
}

func main() {
	var search string

	app := &cli.App{
		Name:        "Gitor",
		Usage:       "Git repo manager",
		Version:     "0.1.0",
		Description: "CLI Tool to manage your bare repos on a remote server.",
		Commands: []*cli.Command{
			{
				Name:    "list",
				Aliases: []string{"ls"},
				Usage:   "List all your repos",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:        "search",
						Aliases:     []string{"s"},
						Usage:       "Search for repos",
						Required:    false,
						DefaultText: "",
						Destination: &search,
					},
				},
				Action: func(c *cli.Context) (err error) {
					repos := getRepositories(search)

					fmt.Printf("Repositories:\n")
					for i := range repos {
						fmt.Printf(strings.Repeat(" ", 2))

						// Regex away the .git suffix
						fmt.Println(
							regexp.MustCompile(`\.git$`).ReplaceAllString(
								repos[i],
								"",
							),
						)
					}
					return
				},
			},
			{
				Name: "repo",
				// Aliases: []string{""},
				Usage: "View a specific repo",
				Action: func(c *cli.Context) (err error) {
					repoName := c.Args().First()
					if repoName == "" {
						fmt.Println("Please specify a repo name")
						return
					}

					repo := getRepository(repoName)
					printRepoInfo(repo)

					return
				},
			},
			{
				Name: "new",
				// Aliases: []string{""},
				Usage: "Create a new repo",
				Action: func(c *cli.Context) (err error) {
					repoName := c.Args().First()
					if repoName == "" {
						fmt.Println("Please specify a repo name")
						return
					}

					repo := newRepository(repoName)
					printRepoInfo(repo)

					return
				},
			},
			{
				Name:    "delete",
				Aliases: []string{"rm"},
				Usage:   "Delete a repo",
				Action: func(c *cli.Context) (err error) {
					repoName := c.Args().First()
					if repoName == "" {
						fmt.Println("Please specify a repo name")
						return
					}

					res := deleteRepository(repoName)
					fmt.Println(res)

					return
				},
			},
		},
	}

	err := app.Run(os.Args)
	check(err)
}

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
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/urfave/cli/v2"
)

var colorReset string = "\033[0m"
var colorGreen string = "\033[32m"
var colorYellow string = "\033[33m"
var colorCyan string = "\033[36m"

// colorPurple := "\033[35m"
// colorRed := "\033[31m"
// colorBlue := "\033[34m"
// colorWhite := "\033[37m"

type ClientConfig struct {
	RemoteServer struct {
		IP    string `yaml:"ip"`
		Port  string `yaml:"port"`
		Token string `yaml:"token"`
	}
}

type Repo struct {
	Name     string
	Branches []string
	Remotes  []string
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
	fmt.Printf("%s%s\n", colorGreen, repo.Name)

	fmt.Printf("\n%s%sBranches:\n", colorYellow, strings.Repeat(" ", 2))
	for i := range repo.Branches {
		fmt.Printf(colorCyan)
		fmt.Printf("    %s\n", repo.Branches[i])
	}

	fmt.Printf("\n%s%sRemotes:\n", colorYellow, strings.Repeat(" ", 2))
	for i := range repo.Remotes {
		fmt.Printf(colorCyan)
		fmt.Printf(
			"    %s\n",
			strings.Replace(
				repo.Remotes[i],
				"\t",
				": ",
				1,
			),
		)
	}

	fmt.Println(colorReset)
}

func encodeToken() string {
	return base64.URLEncoding.EncodeToString(
		[]byte(gitorConfig.RemoteServer.Token),
	)
}

func getRepositories(search string) (result []string) {
	url := fmt.Sprintf(
		"http://%s:%s/%s",
		gitorConfig.RemoteServer.IP,
		gitorConfig.RemoteServer.Port,
		"get_repositories",
	)

	if search != "" {
		url += fmt.Sprintf("?search=%s", search)
	}

	client := &http.Client{
		Transport:     nil,
		CheckRedirect: nil,
		Jar:           nil,
	}
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", encodeToken())
	check(err)

	res, err := client.Do(req)
	check(err)

	body, err := ioutil.ReadAll(res.Body)
	check(err)

	err = json.Unmarshal(body, &result)
	check(err)

	return
}

func getRepository(repoName string) (result Repo) {

	url := fmt.Sprintf(
		"http://%s:%s/%s?%s",
		gitorConfig.RemoteServer.IP,
		gitorConfig.RemoteServer.Port,
		"get_repository",
		fmt.Sprintf("repoName=%s", repoName),
	)
	res, err := http.Get(url)
	check(err)

	body, err := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(body, &result)
	check(err)

	return
}

func newRepository(repoName string) (result Repo) {
	url := fmt.Sprintf(
		"http://%s:%s/%s?%s",
		gitorConfig.RemoteServer.IP,
		gitorConfig.RemoteServer.Port,
		"new_repository",
		fmt.Sprintf("repoName=%s", repoName),
	)

	res, err := http.Get(url)
	check(err)

	body, err := ioutil.ReadAll(res.Body)

	err = json.Unmarshal(body, &result)
	check(err)

	return
}

func main() {
	var search string

	app := &cli.App{
		Name:  "Gitor",
		Usage: "Git repo manager",
		Before: func(c *cli.Context) (err error) {
			// Add some space above our output
			fmt.Printf("\n")
			return
		},
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

					fmt.Printf("%sRepositories:\n", colorYellow)
					for i := range repos {
						fmt.Printf(strings.Repeat(" ", 2))
						fmt.Printf(colorCyan)
						fmt.Println(repos[i])
					}
					fmt.Printf(colorReset)
					return
				},
			},
			{
				Name:    "repo",
				Aliases: []string{"r"},
				Usage:   "View a specific repo",
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
				Name:    "new",
				Aliases: []string{"n"},
				Usage:   "Create a new repo",
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
		},
	}

	err := app.Run(os.Args)
	check(err)
}

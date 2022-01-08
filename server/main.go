package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-yaml/yaml"
)

// Returns the users home directory
func getHomeDir() string {
	homeDir, err := os.UserHomeDir()
	check(err)

	return path.Join(homeDir)
}

var configScriptsFilePath string = path.Join(getHomeDir(), "/.config/gitor/server-config.yml")

type ServerConfig struct {
	Paths struct {
		Repositories string `yaml:repositories`
	}
	Server struct {
		Port           string   `yaml:port`
		TokenWhitelist []string `yaml:tokenWhitelist`
	}
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getConfig() (cfg ServerConfig) {
	f, err := os.Open(configScriptsFilePath)
	check(err)
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	check(err)

	f.Close()

	return
}

func validateToken(token string) bool {
	tokenWhiteList := getConfig().Server.TokenWhitelist
	decodedToken, err := base64.StdEncoding.DecodeString(token)
	check(err)

	for i := range tokenWhiteList {
		if string(decodedToken) == tokenWhiteList[i] {
			return true
		}
	}

	return false
}

func getRepositories(res http.ResponseWriter, req *http.Request) {
	if !validateToken(req.Header.Get("Authorization")) {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte("401 Unauthorized"))
		return
	}

	// Find all the git repos
	dirs, err := os.ReadDir(getConfig().Paths.Repositories)
	check(err)

	// Make a list of all the name of the git repos
	var data []string
	for i := range dirs {
		if dirs[i].IsDir() {
			data = append(data, dirs[i].Name())
		}
	}

	encode := json.NewEncoder(res)
	encode.Encode(data)
}

type Repo struct {
	Name     string
	Branches []string
	Remotes  []string
}

func getRepository(res http.ResponseWriter, req *http.Request) {

	repoName := req.URL.Query().Get("repoName")
	repoPath := path.Join(getConfig().Paths.Repositories, repoName)

	var repoRes = &Repo{
		Name: repoName,
	}

	repo, err := git.PlainOpen(repoPath)
	check(err)

	/// Get all the branches
	branchesIter, err := repo.Branches()
	check(err)
	branchesIter.ForEach(func(r *plumbing.Reference) error {
		branch := strings.Replace(string(r.Name()), "refs/heads/", "", 1)
		repoRes.Branches = append(repoRes.Branches, branch)
		return nil
	})

	// Get all the remotes
	remotesIter, err := repo.Remotes()
	check(err)
	for i := range remotesIter {
		remote := remotesIter[i].String()
		remoteList := strings.Split(remote, "\n")
		repoRes.Remotes = append(repoRes.Remotes, remoteList...)
	}

	encode := json.NewEncoder(res)
	encode.Encode(repoRes)
}

func newRepository(res http.ResponseWriter, req *http.Request) {
	repoName := req.URL.Query().Get("repoName")

	gitorConfig := getConfig()
	repo, err := git.PlainInit(path.Join(gitorConfig.Paths.Repositories, repoName), true)
	check(err)

	err = repo.CreateBranch(&config.Branch{
		Name:   repoName,
		Remote: "",
		Merge:  "",
		Rebase: "",
	})
	check(err)

	encode := json.NewEncoder(res)
	encode.Encode(repo)
}

func main() {
	gitorConfig := getConfig()

	http.HandleFunc("/get_repositories", getRepositories)
	http.HandleFunc("/get_repository", getRepository)
	http.HandleFunc("/new_repository", newRepository)

	http.ListenAndServe(fmt.Sprintf(":%s", gitorConfig.Server.Port), nil)
}

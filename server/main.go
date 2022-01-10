package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-yaml/yaml"
)

type Repo struct {
	Name     string
	Branches []string
	Remotes  []string
}

type ServerConfig struct {
	Paths struct {
		Repositories string `yaml:"repositories"`
	}
	Server struct {
		Port           string   `yaml:"port"`
		TokenWhitelist []string `yaml:"tokenWhitelist"`
		SSHPort        string   `yaml:"sshPort"`
		Address        string   `yaml:"address"`
		User           string   `yaml:"user"`
	}
}

// Returns the users home directory
func getHomeDir() string {
	homeDir, err := os.UserHomeDir()
	check(err)

	return path.Join(homeDir)
}

var configScriptsFilePath string = path.Join(getHomeDir(), "/.config/gitor/server-config.yml")

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getConfig() (cfg *ServerConfig) {
	f, err := os.Open(configScriptsFilePath)
	check(err)
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	check(err)

	f.Close()

	return
}

var gitorConfig ServerConfig = *getConfig()

func validateToken(token string) bool {
	tokenWhiteList := gitorConfig.Server.TokenWhitelist
	decodedToken, err := base64.StdEncoding.DecodeString(token)
	check(err)

	// Check if the decode token is in the whitelist
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

	search := req.URL.Query().Get("search")

	// Find all the git repos
	dirs, err := os.ReadDir(gitorConfig.Paths.Repositories)
	check(err)

	// Make a list of all the name of the git repos
	var data []string
	for i := range dirs {
		if dirs[i].IsDir() {
			if search != "" {
				if strings.Contains(dirs[i].Name(), search) {
					data = append(data, dirs[i].Name())
				}
			} else {
				data = append(data, dirs[i].Name())
			}
		}
	}

	encode := json.NewEncoder(res)
	encode.Encode(data)
}

func getRepository(res http.ResponseWriter, req *http.Request) {
	if !validateToken(req.Header.Get("Authorization")) {
		res.WriteHeader(http.StatusUnauthorized)
		res.Write([]byte("401 Unauthorized"))
		return
	}

	repoName := req.URL.Query().Get("repoName")
	repoPath := path.Join(gitorConfig.Paths.Repositories, repoName+".git")

	// Check if the directory exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		res.WriteHeader(http.StatusNotFound)
		res.Write([]byte("404 Not Found"))
		return
	}

	var repoRes = &Repo{
		Name: repoName,
	}

	// Open the repo so we can inspect it
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
	if !validateToken(req.Header.Get("Authorization")) {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte("401 Unauthorized"))
		return
	}

	repoName := req.URL.Query().Get("repoName")
	repoPath := path.Join(gitorConfig.Paths.Repositories, repoName+".git")

	// Make the repo
	repo, err := git.PlainInit(repoPath, true)
	check(err)

	repoRes := Repo{
		Name:     repoName,
		Branches: []string{},
	}

	// Create origin remote
	remote, err := repo.CreateRemote(&config.RemoteConfig{
		Name: "origin",
		URLs: []string{
			fmt.Sprintf(
				"ssh://%s@%s:%s%s",
				gitorConfig.Server.User,
				gitorConfig.Server.Address,
				gitorConfig.Server.SSHPort,
				repoPath,
			),
		},
		Fetch: []config.RefSpec{},
	})
	check(err)

	// Chown the repo
	// TODO: Can we avoid this?
	os.Chown(repoPath, os.Getuid(), os.Getgid())
	cmd := exec.Command(
		"bash",
		"-c",
		fmt.Sprintf(
			"chown -R %s:%s %s",
			gitorConfig.Server.User,
			gitorConfig.Server.User,
			repoPath,
		),
	)
	err = cmd.Run()
	check(err)

	// Add the remote to repoRes
	remoteString := remote.String()
	remoteList := strings.Split(remoteString, "\n")
	repoRes.Remotes = append(repoRes.Remotes, remoteList...)

	encode := json.NewEncoder(res)
	encode.Encode(repoRes)
}

func deleteRepository(res http.ResponseWriter, req *http.Request) {
	if !validateToken(req.Header.Get("Authorization")) {
		res.WriteHeader(http.StatusUnauthorized)
		res.Write([]byte("401 Unauthorized"))
		return
	}

	repoName := req.URL.Query().Get("repoName")
	repoPath := path.Join(gitorConfig.Paths.Repositories, repoName+".git")

	// Check if exists
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		res.WriteHeader(http.StatusNotFound)
		res.Write([]byte("404 Not Found"))
		return
	}

	// Delete the directory
	err := os.RemoveAll(repoPath)

	// Check if everything went well
	if err == nil {
		encode := json.NewEncoder(res)
		encode.Encode(fmt.Sprintf("%s Has been deleted", repoName))
	} else {
		res.WriteHeader(http.StatusInternalServerError)
		res.Write([]byte("500 Something went wrong"))
		return
	}
}

func main() {
	http.HandleFunc("/get_repositories", getRepositories)
	http.HandleFunc("/get_repository", getRepository)
	http.HandleFunc("/new_repository", newRepository)
	http.HandleFunc("/delete_repository", deleteRepository)

	http.ListenAndServe(fmt.Sprintf(":%s", gitorConfig.Server.Port), nil)
}

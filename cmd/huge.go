/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

const hugeCommitsCount = "count"
const path = "path"
const verbose = "verbose"

var windowsGitBashPath string

// hugeCmd represents the huge command
var hugeCmd = &cobra.Command{
	Use:   "huge",
	Short: "clean huge commit",
	Long:  `clean local repository huge commit`,
	Run:   hugeRun,
}

func init() {
	checkWindowsGitPath()
	rootCmd.AddCommand(hugeCmd)
	hugeCmd.Flags().StringP(path, "p", "", "local git repo path.(need absolute path)")
	hugeCmd.Flags().IntP(hugeCommitsCount, "c", 10, "query git huge commits count")
	hugeCmd.Flags().BoolP(verbose, "v", false, "verbose info")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// hugeCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// hugeCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func checkWindowsGitPath() {
	if runtime.GOOS == "windows" {
		envList := os.Getenv("path")
		split := strings.Split(envList, ";")
		for _, s := range split {
			if strings.HasSuffix(s, "Git\\cmd") {
				windowsGitBashPath = strings.Replace(s, "cmd", "git-bash", 1)
				fmt.Println("found git bash path:" + windowsGitBashPath)
				break
			}
		}
		if windowsGitBashPath == "" {
			fmt.Println("not found git-bash in windows path.please check has installed git??")
			os.Exit(1)
		}
	}
}

func hugeRun(cmd *cobra.Command, args []string) {
	flags := cmd.Flags()
	hugeCount, err2 := flags.GetInt(hugeCommitsCount)
	if err2 != nil {
		fmt.Println("invalid hugeCommitsCount arg" + err2.Error())
		return
	}
	verbose, err2 := flags.GetBool(verbose)
	if err2 != nil {
		fmt.Println("invalid verbose arg" + err2.Error())
		return
	}
	repoPath, err2 := flags.GetString(path)
	if err2 != nil {
		fmt.Println("invalid path arg" + err2.Error())
		return
	}
	if strings.EqualFold("", repoPath) {
		fmt.Println("path can not be empty")
		return
	}
	fmt.Println("----environment info----")
	fmt.Println("os:" + runtime.GOOS)
	gitVersion := `git version`
	_, _, err := Exec(repoPath, gitVersion, true)
	if err != nil {
		fmt.Println("git version failed.please check has install git??" + err.Error())
		return
	}
	fmt.Println("----git objects size----")
	gitCount := `git count-objects -vH`
	_, _, err = Exec(repoPath, gitCount, true)
	if err != nil {
		fmt.Println("git calc objects count failed:" + err.Error())
		return
	}
	fmt.Println("----query top " + strconv.Itoa(hugeCount) + " huge commits----")
	gitShowTemplate := `git rev-list --objects --all | grep "$(git verify-pack -v .git/objects/pack/*.idx | sort -k 3 -n | tail -(count) | awk '{print$1}')"`
	gitShow := strings.Replace(gitShowTemplate, "(count)", strconv.Itoa(hugeCount), 1)
	_, result, err := Exec(repoPath, gitShow, true)
	if err != nil {
		fmt.Println("git rev-list failed:" + err.Error())
		return
	}
	objectsInfoList, err := getGCInfos(repoPath, result, verbose)
	if err != nil {
		fmt.Println("cat objects size failed:" + err.Error())
	}
	fmt.Println("------- please select indexes for clean -------")
	for index, gcInfo := range objectsInfoList {
		fmt.Printf("%v)\t%v\t%v\t%v\n", index+1, gcInfo.SHA, gcInfo.Path, gcInfo.Size)
	}
	scanner := bufio.NewScanner(os.Stdin)
	needsCleanObjects := []GCInfo{}
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			continue
		}
		if strings.EqualFold("y", text) {
			break
		}
		if strings.EqualFold("n", text) {
			return
		}
		index, err := strconv.Atoi(text)
		if err != nil {
			fmt.Println("invalid input:" + text + "error:" + err.Error())
			continue
		}
		if index > len(objectsInfoList) {
			fmt.Println("selected index cannot greater than objects size")
			continue
		}
		info := objectsInfoList[index-1]
		needsCleanObjects = append(needsCleanObjects, info)
		pathList := make([]string, len(needsCleanObjects))
		for i, g := range needsCleanObjects {
			pathList[i] = g.Path
		}
		fmt.Println("selected:\n" + strings.Join(pathList, "\n"))
	}
	for _, info := range needsCleanObjects {
		cleanCommandTemplate := `git filter-branch --force --index-filter 'git rm -rq --cached --ignore-unmatch "%v"' -- --all`
		cleanCommand := fmt.Sprintf(cleanCommandTemplate, strings.TrimSpace(info.Path))
		if verbose {
			fmt.Println("start clean branches,command:[" + cleanCommand + "]")
		}
		_, _, err := Exec(repoPath, cleanCommand, true)
		if err != nil {
			fmt.Println("git filter-branch clean branch failed." + err.Error())
			return
		}
		cleanTagCommandTemplate := `git filter-branch --force --tag-name-filter cat --index-filter 'git rm -rq --cached --ignore-unmatch "%v"' -- --all`
		cleanTagCommand := fmt.Sprintf(cleanTagCommandTemplate, strings.TrimSpace(info.Path))
		if verbose {
			fmt.Println("start clean tags,command:[" + cleanTagCommand + "]")
		}
		_, _, err = Exec(repoPath, cleanTagCommand, true)
		if err != nil {
			fmt.Println("git filter-branch clean tags failed." + err.Error())
			return
		}
	}
	cleanLogCommand := `rm -rf .git/logs/`
	if verbose {
		fmt.Println("will execute clean log command:[" + cleanLogCommand + "]")
	}
	_, _, err2 = Exec(repoPath, cleanLogCommand, verbose)
	if err2 != nil {
		fmt.Println("clean local git logs failed." + err2.Error())
	}
	gcCommand := `git gc`
	if verbose {
		fmt.Println("will execute git gc command:[" + gcCommand + "]")
	}
	_, _, err2 = Exec(repoPath, gcCommand, verbose)
	if err2 != nil {
		fmt.Println("git gc failed." + err2.Error())
	}
	pruneCommand := `git prune`
	if verbose {
		fmt.Println("will execute git prune command:[" + pruneCommand + "]")
	}
	_, _, err2 = Exec(repoPath, pruneCommand, verbose)
	if err2 != nil {
		fmt.Println("clean local git logs failed." + err2.Error())
	}
}

func getGCInfos(dir, result string, verbose bool) ([]GCInfo, error) {
	result = strings.TrimSpace(result)
	split := strings.Split(result, "\n")
	gcInfos := make(GCInfoSlice, len(split))
	for index, line := range split {
		runes := []rune(line)
		sha := string(runes[:40])
		filePath := string(runes[40:])
		catCommand := "git cat-file -s " + sha
		_, s, err := Exec(dir, catCommand, verbose)
		if err != nil {
			return nil, err
		}
		size, err := mbSize(s)
		if err != nil {
			return nil, err
		}
		i, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		gcInfo := GCInfo{SHA: sha, Path: filePath, Size: size, Byte: i}
		if verbose {
			fmt.Printf("sha:%v\tpath:%v\tMbSize:%v\tbyteSize:%v\n", gcInfo.SHA, gcInfo.Path, gcInfo.Size, gcInfo.Byte)
		}
		gcInfos[index] = gcInfo
	}
	sort.Sort(gcInfos)
	return gcInfos, nil
}

func mbSize(bytesSize string) (string, error) {
	bytesSize = strings.TrimSpace(bytesSize)
	parseInt, err := strconv.ParseInt(bytesSize, 10, 64)
	if err != nil {
		return "", err
	}
	i := parseInt / 1000 / 1000
	return strconv.FormatInt(i, 10) + "Mb", nil
}
func Exec(dir, commandString string, isStdout bool) (*exec.Cmd, string, error) {
	command := exec.Command("bash", "-c", commandString)
	buf := new(bytes.Buffer)
	var writer io.Writer
	if isStdout {
		writer = io.MultiWriter(os.Stdout, buf)
	} else {
		writer = io.MultiWriter(buf)
	}
	command.Stdout = writer
	command.Stderr = writer
	command.Dir = dir
	err := command.Run()
	if err != nil {
		return nil, "", err
	}
	return command, buf.String(), nil
}

type GCInfo struct {
	SHA  string
	Path string
	Size string
	Byte int
}

type GCInfoSlice []GCInfo

func (g GCInfoSlice) Len() int {
	return len(g)
}

func (g GCInfoSlice) Less(i, j int) bool {
	return g[i].Byte > g[j].Byte
}

func (g GCInfoSlice) Swap(i, j int) {
	g[i], g[j] = g[j], g[i]
}

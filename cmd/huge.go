package cmd

import (
	"bufio"
	"bytes"
	"clean-git/utils"
	"fmt"
	"github.com/spf13/cobra"
	"io"
	"os"
	"os/exec"
	"os/user"
	os_path "path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const hugeCommitsCount = "count"
const path = "path"
const verbose = "verbose"
const version = "1.0.1"

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
				utils.BlueLnFunc("found git bash path:" + windowsGitBashPath)
				break
			}
		}
		if windowsGitBashPath == "" {
			utils.RedlnFunc("not found git-bash in windows path.please check has installed git??")
			os.Exit(1)
		}
	}
}

func hugeRun(cmd *cobra.Command, args []string) {
	// ignore git warning
	os.Setenv("FILTER_BRANCH_SQUELCH_WARNING", "1")
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
	if strings.EqualFold("", strings.TrimSpace(repoPath)) {
		fmt.Println("path can not be empty")
		return
	}
	showEnvironmentInfo(repoPath, verbose)
	warning := `
=============			Warning			==============
..............Please Back Up Repository...............
..............Please Back Up Repository...............
..............Please Back Up Repository...............
=============			Warning			==============
	`
	fmt.Println(utils.RedStr(warning))
	warningSca := bufio.NewScanner(os.Stdin)
	fmt.Println(utils.BlueStr("please enter 'y' for confirm. has backup repository"))
	for warningSca.Scan() {
		text := warningSca.Text()
		if text == "" {
			continue
		}
		if "y" == text {
			break
		}
		if "n" == text {
			return
		}
	}
	fmt.Println(utils.BlueStr("---- calc git objects size ----"))
	gitCount := `git gc && git count-objects -vH`
	fmt.Println("It may take a while, please wait.......")
	var err error
	_, _, err = Exec(repoPath, gitCount, true)
	if err != nil {
		fmt.Println("git calc objects count failed:" + err.Error())
		return
	}
	fmt.Println(utils.BlueStr("----query top " + strconv.Itoa(hugeCount) + " huge commits----"))
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
	utils.BlueLnFunc("------- please select indexes for clean -------")
	for index, gcInfo := range objectsInfoList {
		fmt.Printf(utils.BlueFStr("%v) ", index+1)+"\t%v\t%v\t%v\t%v\n", gcInfo.SHA, gcInfo.Type, gcInfo.Path, gcInfo.HumanSize)
	}
	scanner := bufio.NewScanner(os.Stdin)
	var needsCleanObjects []GCInfo
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
			utils.RedStr("invalid input:" + text + "error:" + err.Error())
			continue
		}
		if index > len(objectsInfoList) {
			utils.RedlnFunc("selected index cannot greater than objects size")
			continue
		}
		info := objectsInfoList[index-1]
		needsCleanObjects = append(needsCleanObjects, info)
		pathList := make([]string, len(needsCleanObjects))
		for i, g := range needsCleanObjects {
			pathList[i] = g.Path
		}
		utils.RedlnFunc("selected:\n" + strings.Join(pathList, "\n") + "\nEnter 'y' for confirm. Enter 'n' for cancel")
	}
	fmt.Println(utils.BlueStr("will begin clean huge commits.... please wait...."))
	remoteName := "clean"
	err = prepareRemote(repoPath, remoteName, verbose)
	if err != nil {
		fmt.Println("prepare remote failed." + err.Error())
		return
	}
	startTime := time.Now()
	// prepare path for fast clean
	pathList := make([]string, 0)
	for _, info := range needsCleanObjects {
		pathList = append(pathList, `"`+info.Path+`"`)
	}
	fullPaths := strings.Join(pathList, " ")
	gitFilterBranchTemplate := labelCommand{
		label:  "git filter-branch for all branches",
		script: `git filter-branch --prune-empty --force --index-filter 'git rm -rf --cached --ignore-unmatch (path)' -- --all`,
	}
	gitFilterTemplates := []labelCommand{gitFilterBranchTemplate}
	tag, err := hasTag(repoPath, verbose)
	if tag {
		gitFilterTagTemplate := labelCommand{
			label:  "git filter-branch for tag",
			script: `git filter-branch --prune-empty --force --tag-name-filter cat --index-filter 'git rm -rf --cached --ignore-unmatch (path)' -- --all`,
		}
		gitFilterTemplates = append(gitFilterTemplates, gitFilterTagTemplate)
	}
	for _, template := range gitFilterTemplates {
		shell := strings.Replace(template.script, "(path)", strings.TrimSpace(fullPaths), 1)
		if verbose {
			fmt.Println("DEBUG: label [" + template.label + "]. command [" + shell + "]")
		}
		_, _, e := Exec(repoPath, shell, true)
		if e != nil {
			utils.RedlnFunc("label [" + template.label + "]. failed execute [" + shell + "]. error:" + e.Error())
			return
		}
	}
	gitLogPath := os_path.Join(repoPath, ".git/logs/")
	err = utils.DelGitFile(gitLogPath)
	if err != nil {
		fmt.Println("delete git logs failed." + err.Error())
		return
	}
	gitOriginalPath := os_path.Join(repoPath, ".git/refs/original")
	err = utils.DelGitFile(gitOriginalPath)
	if err != nil {
		fmt.Println("delete git ref original failed." + err.Error())
		return
	}
	gitGcLabel := labelCommand{label: "git gc", script: `git gc --aggressive --prune=now`}
	orderedLabelCmd := []labelCommand{gitGcLabel}
	for _, labelCmd := range orderedLabelCmd {
		if e := cleanGitLog(repoPath, labelCmd, verbose); e != nil {
			fmt.Println("label [" + labelCmd.label + "].execute [" + labelCmd.script + "] failed." + e.Error())
			return
		}
	}
	endTime := time.Now()
	duration := endTime.Sub(startTime)
	fmt.Println(utils.BlueStr("------- clean complete -------"))
	fmt.Println(utils.BlueStr("Time:\t" + duration.String()))
	fmt.Println(utils.BlueStr("1. use command  `git push " + remoteName + " --all -f` apply to remote repository branches"))
	if tag {
		fmt.Println(utils.BlueStr("2. use command  `git push " + remoteName + " --tags -f` apply to remote repository tags"))
	}
}

func prepareRemote(repoPath, remoteName string, verbose bool) error {
	renameRemote := labelCommand{
		label:  "rename remote",
		script: "git remote rename origin " + remoteName,
	}
	cpOriginRemote := labelCommand{
		label:  "copy remote branches",
		script: "cp -R .git/refs/remotes/" + remoteName + "/* .git/refs/heads/",
	}
	delHead := labelCommand{
		label:  "delete head",
		script: "rm -rf .git/refs/heads/HEAD",
	}
	labelCommands := []labelCommand{renameRemote, cpOriginRemote, delHead}
	for _, cmd := range labelCommands {
		if verbose {
			fmt.Println("DEBUG: label [" + cmd.label + "]. command:[" + cmd.script + "]")
		}
		_, _, err := Exec(repoPath, cmd.script, verbose)
		if err != nil {
			return err
		}
	}
	return nil
}

// clean git log. eg: ref、logs
func cleanGitLog(repoPath string, labelCmd labelCommand, verbose bool) error {
	if verbose {
		fmt.Println("label [" + labelCmd.label + "]. will clean git. command:[" + labelCmd.script + "]")
	}
	_, _, err := Exec(repoPath, labelCmd.script, verbose)
	if err != nil {
		return err
	}
	return nil
}

func getGCInfos(dir, result string, verbose bool) ([]GCInfo, error) {
	result = strings.TrimSpace(result)
	split := strings.Split(result, "\n")
	gcInfos := make(GCInfoSlice, 0)
	for _, line := range split {
		runes := []rune(line)
		sha := string(runes[:40])
		filePath := strings.TrimSpace(string(runes[40:]))
		humanSize, byteSize, err := getCommitSize(dir, sha, verbose)
		if err != nil {
			return nil, err
		}
		commitType, err := getCommitType(dir, sha, verbose)
		if err != nil {
			return nil, err
		}
		gcInfo := GCInfo{
			SHA:       sha,
			Path:      filePath,
			HumanSize: humanSize,
			Byte:      byteSize,
			Type:      commitType,
		}
		if verbose {
			fmt.Printf("sha:%v\ttype:%v\tpath:%v\tMbSize:%v\tbyteSize:%v\n", gcInfo.SHA, gcInfo.Type, gcInfo.Path, gcInfo.HumanSize, gcInfo.Byte)
		}
		if gcInfo.Path != "" {
			gcInfos = append(gcInfos, gcInfo)
		}
	}
	sort.Sort(gcInfos)
	return gcInfos, nil
}

func byte2HumanSize(bytesSize int64) (string, error) {
	kb := bytesSize / 1000
	if kb == 0 {
		return strconv.FormatInt(bytesSize, 10) + "Byte", nil
	}
	mb := kb / 1000
	if mb == 0 {
		return strconv.FormatInt(kb, 10) + "Kb", nil
	}
	gb := mb / 1000
	if gb == 0 {
		return strconv.FormatInt(mb, 10) + "Mb", nil
	}
	return strconv.FormatInt(gb, 10) + "Gb", nil
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
	SHA       string
	Path      string
	HumanSize string
	Byte      int64
	// tree、tag、blob...
	Type string
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

type labelCommand struct {
	label  string
	script string
}

func showEnvironmentInfo(repoPath string, verbose bool) {
	utils.BlueLnFunc("---- Environment Info ----")
	fmt.Println(utils.BlueStr("OS:\t") + runtime.GOOS)
	currentUser, err := user.Current()
	if err != nil {
		utils.RedlnFunc("query user name failed." + err.Error())
		return
	}
	fmt.Println(utils.BlueStr("Name:\t") + currentUser.Username)
	fmt.Println(utils.BlueStr("GitClean Version:\t") + version)
	gitVersion := `git version`
	_, versionInfo, err := Exec(repoPath, gitVersion, false)
	versionInfo = strings.Replace(versionInfo, "git", "", 1)
	versionInfo = strings.Replace(versionInfo, "version", "", 1)
	versionInfo = strings.TrimSpace(versionInfo)
	if err != nil {
		utils.RedlnFunc("get git version failed.please check has installed git??" + err.Error())
		return
	}
	fmt.Println(utils.BlueStr("Git Version:\t") + versionInfo)
	fmt.Println(utils.BlueStr("Verbose:\t") + strconv.FormatBool(verbose))
}

// getCommitSize
func getCommitSize(repoPath, sha string, verbose bool) (string, int64, error) {
	catCommand := "git cat-file -s " + sha
	_, byteSize, err := Exec(repoPath, catCommand, verbose)
	if err != nil {
		return "", 0, err
	}
	byteSize = strings.TrimSpace(byteSize)
	intByteSize, err := strconv.ParseInt(byteSize, 10, 64)
	if err != nil {
		return "", 0, err
	}
	humanSize, err := byte2HumanSize(intByteSize)
	if err != nil {
		return "", 0, err
	}
	return humanSize, intByteSize, nil
}

func getCommitType(repoPath, sha string, verbose bool) (string, error) {
	catCommand := "git cat-file -t " + sha
	_, result, err := Exec(repoPath, catCommand, verbose)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(result), nil
}

// hasTag current repo has tag
func hasTag(repoPath string, verbose bool) (bool, error) {
	command := `git tag`
	_, result, err := Exec(repoPath, command, false)
	if err != nil {
		return false, err
	}
	if verbose {
		fmt.Println("tag list is [" + result + "]")
	}
	return strings.TrimSpace(result) != "", nil
}

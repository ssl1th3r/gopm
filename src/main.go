package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var defaultRepo = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages/packages.json"
var urllatestjson = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/releases/latest.json"
var checkUpdateFlag = true
var binBase string

var version = "0.0.3"
var logo = `
 _____       ____  __  __ 
/  ___| ___ |  _ \|  \/  |
| |  _ / _ \| |_) | |\/| |
| |_| | (_) |  __/| |  | | 
 \____|\___/|_|   |_|  |_|
`

type ReleaseInfo struct {
	Version string `json:"version"`
	Binary  string `json:"binary"`
}

type PackageInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

const (
	RED    = "\033[31m"
	GREEN  = "\033[32m"
	YELLOW = "\033[33m"
	BLUE   = "\033[34m"
	RESET  = "\033[0m"
)

var cachedPackages []PackageInfo
var repos []string
var configPath string
var installedPackagesFile string
var installedPackages []PackageInfo

func main() {
	initConfig()
	loadInstalledPackages()
	if checkUpdateFlag {
		checkUpdate()
	}

	if len(os.Args) < 2 {
		printHelp()
		return
	}

	switch os.Args[1] {
	case "info":
		fmt.Println(logo)
		fmt.Println("Version:", version)

	case "dwld":
		handleDownload(os.Args[2:])

	case "upd":
		updatePkg(os.Args[2:])

	case "updateall":
		updateAllPackages()

	case "remove":
		removePkg(os.Args[2:])

	case "updatepm":
		updateSelf()

	case "list":
		listAvailable()

	case "search":
		if len(os.Args) < 3 {
			fmt.Println("Usage: gopm search <name>")
			return
		}
		searchPackage(os.Args[2])

	case "set":
		handleSet(os.Args[2:])

	default:
		printHelp()
	}
}

// config
func initConfig() {
	usr, err := user.Current()
	if err != nil {
		fmt.Println(RED+"Failed to get user dir:"+RESET, err)
		repos = []string{defaultRepo}
		binBase = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages"
		checkUpdateFlag = true
		return
	}

	configPath = filepath.Join(usr.HomeDir, ".config", "gopm.conf")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(configPath), 0755)
		content := fmt.Sprintf(
			"repos=%s\nbin_base=%s\ncheck_update=true\n",
			defaultRepo,
			"https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages",
		)
		os.WriteFile(configPath, []byte(content), 0644)
		repos = []string{defaultRepo}
		binBase = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages"
		checkUpdateFlag = true
		return
	}

	file, err := os.Open(configPath)
	if err != nil {
		fmt.Println(RED+"Failed to open config:"+RESET, err)
		repos = []string{defaultRepo}
		binBase = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages"
		checkUpdateFlag = true
		return
	}
	defer file.Close()

	repos = []string{}
	binBase = ""
	checkUpdateFlag = true

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "repos=") {
			val := strings.TrimPrefix(line, "repos=")
			for _, r := range strings.Split(val, ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					repos = append(repos, r)
				}
			}
		} else if strings.HasPrefix(line, "bin_base=") {
			binBase = strings.TrimSpace(strings.TrimPrefix(line, "bin_base="))
		} else if strings.HasPrefix(line, "check_update=") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "check_update="))
			checkUpdateFlag = strings.ToLower(val) == "true"
		}
	}

	if len(repos) == 0 {
		repos = []string{defaultRepo}
	}
	if binBase == "" {
		binBase = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages"
	}

	installedPackagesFile = filepath.Join(usr.HomeDir, ".config", "gopm_installed.json")
}

func saveConfig() error {
	os.MkdirAll(filepath.Dir(configPath), 0755)
	content := "repos=" + strings.Join(repos, ",") + "\n"
	content += "bin_base=" + binBase + "\n"
	content += "check_update=" + strconv.FormatBool(checkUpdateFlag) + "\n"
	return os.WriteFile(configPath, []byte(content), 0644)
}

// installed packages
func loadInstalledPackages() {
	if data, err := os.ReadFile(installedPackagesFile); err == nil {
		json.Unmarshal(data, &installedPackages)
	}
}

func saveInstalledPackages() {
	data, _ := json.MarshalIndent(installedPackages, "", "  ")
	os.MkdirAll(filepath.Dir(installedPackagesFile), 0755)
	os.WriteFile(installedPackagesFile, data, 0644)
}

// set command
func handleSet(args []string) {
	if len(args) < 2 {
		fmt.Println("Usage: gopm set <repos|check_update> <value>")
		return
	}

	switch args[0] {
	case "repos":
		repos = strings.Split(args[1], ",")
		fmt.Println(GREEN+"Repos updated:"+RESET, repos)
	case "check_update":
		checkUpdateFlag = strings.ToLower(args[1]) == "true"
		fmt.Println(GREEN+"check_update set to:"+RESET, checkUpdateFlag)
	default:
		fmt.Println(RED+"Unknown config key:"+RESET, args[0])
		return
	}

	if err := saveConfig(); err != nil {
		fmt.Println(RED+"Failed to save config:"+RESET, err)
	}
}

// update
func checkUpdate() {
	resp, err := http.Get(urllatestjson)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var r ReleaseInfo
	if json.NewDecoder(resp.Body).Decode(&r) == nil {
		if r.Version != version {
			fmt.Println(YELLOW+"Update available:"+RESET, r.Version)
		}
	}
}

func updateSelf() {
	url := "https://github.com/ssl1th3r/gopmpackagesupdate/raw/main/bin/gopm"

	if !confirm("Update gopm?") {
		return
	}

	fmt.Println(BLUE + "Downloading gopm update..." + RESET)
	if err := downloadAndInstall(url, "gopm"); err != nil {
		fmt.Println(RED+"Update failed:"+RESET, err)
		return
	}
	fmt.Println(GREEN + "gopm updated" + RESET)
}

// package system
func handleDownload(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: gopm dwld <pkg> [--ver X]")
		return
	}

	name := args[0]
	ver := "latest"

	for i := 0; i < len(args); i++ {
		if args[i] == "--ver" && i+1 < len(args) {
			ver = args[i+1]
		}
	}

	if strings.HasPrefix(ver, ">") {
		ver = resolveVersion(name, ver)
	}

	installPkg(name, ver)
}

func updatePkg(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: gopm upd <pkg>")
		return
	}

	fmt.Println(BLUE+"Updating package:"+RESET, args[0])
	installPkg(args[0], "latest")
}

func updateAllPackages() {
	if len(installedPackages) == 0 {
		fmt.Println(YELLOW + "No packages installed." + RESET)
		return
	}

	for _, p := range installedPackages {
		fmt.Println(BLUE+"Updating package:"+RESET, p.Name)
		installPkg(p.Name, "latest")
	}
}

func installPkg(name, ver string) {
	if len(repos) == 0 || binBase == "" {
		fmt.Println(RED + "No repos/bin_base defined" + RESET)
		return
	}

	url := fmt.Sprintf("%s/%s/%s/%s", binBase, name, ver, name)

	fmt.Println(BLUE+"Package:"+RESET, name)
	fmt.Println(BLUE+"Version:"+RESET, ver)
	fmt.Println(BLUE+"URL:"+RESET, url)
	fmt.Println(BLUE+"OS:"+RESET, runtime.GOOS)

	if !confirm("Proceed with install/update?") {
		return
	}

	if err := downloadAndInstall(url, name); err != nil {
		fmt.Println(RED+"Install failed:"+RESET, err)
		return
	}

	found := false
	for i, p := range installedPackages {
		if p.Name == name {
			installedPackages[i].Version = ver
			found = true
			break
		}
	}
	if !found {
		installedPackages = append(installedPackages, PackageInfo{Name: name, Version: ver})
	}

	saveInstalledPackages()
	fmt.Println(GREEN+"Installed:"+RESET, name)
}

func downloadAndInstall(url, name string) error {
	dest := "/usr/bin/" + name

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}

	tmp := filepath.Join(os.TempDir(), name)
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	defer out.Close()

	printDownloadProgress(resp.Body, out, resp.ContentLength)
	out.Sync()
	os.Chmod(tmp, 0755)

	in, err := os.Open(tmp)
	if err != nil {
		return err
	}
	defer in.Close()

	outDest, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer outDest.Close()

	if _, err := io.Copy(outDest, in); err != nil {
		return err
	}
	outDest.Sync()

	return os.Remove(tmp)
}

func searchPackage(query string) {
	pkgs, err := fetchPackages()
	if err != nil {
		fmt.Println(RED + "Failed to fetch package list" + RESET)
		return
	}

	found := false
	for _, p := range pkgs {
		if strings.Contains(strings.ToLower(p.Name), strings.ToLower(query)) {
			fmt.Printf("%s%s%s (%s)\n", GREEN, p.Name, RESET, p.Version)
			found = true
		}
	}

	if !found {
		fmt.Println(YELLOW+"No packages found for query:"+RESET, query)
	}
}

func printDownloadProgress(src io.Reader, dst io.Writer, total int64) {
	buf := make([]byte, 32*1024)
	var done int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			dst.Write(buf[:n])
			done += int64(n)
			if total > 0 {
				fmt.Printf("\rDownloading... %.1f%%", float64(done)/float64(total)*100)
			}
		}
		if err != nil {
			fmt.Println()
			return
		}
	}
}

func removePkg(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: gopm remove <pkg>")
		return
	}

	if !confirm("Remove " + args[0] + "?") {
		return
	}

	os.Remove("/usr/bin/" + args[0])

	for i, p := range installedPackages {
		if p.Name == args[0] {
			installedPackages = append(installedPackages[:i], installedPackages[i+1:]...)
			break
		}
	}
	saveInstalledPackages()
	fmt.Println(GREEN+"Removed:"+RESET, args[0])
}

func resolveVersion(pkg, cond string) string {
	pkgs, err := fetchPackages()
	if err != nil {
		return "latest"
	}

	base := strings.Trim(cond, ">=")
	best := base

	for _, p := range pkgs {
		if p.Name == pkg && compareVersion(p.Version, best) {
			best = p.Version
		}
	}
	return best
}

func compareVersion(a, b string) bool {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")

	max := len(as)
	if len(bs) > max {
		max = len(bs)
	}

	for i := 0; i < max; i++ {
		ai, bi := 0, 0
		if i < len(as) {
			ai, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			bi, _ = strconv.Atoi(bs[i])
		}
		if ai != bi {
			return ai > bi
		}
	}
	return true
}

func listAvailable() {
	pkgs, err := fetchPackages()
	if err != nil {
		return
	}

	for _, p := range pkgs {
		fmt.Printf("%s%s%s (%s)\n", GREEN, p.Name, RESET, p.Version)
	}
}

func confirm(msg string) bool {
	fmt.Print(YELLOW + msg + " [y/N]: " + RESET)
	var in string
	fmt.Scanln(&in)
	return strings.ToLower(in) == "y"
}

func fetchPackages() ([]PackageInfo, error) {
	if cachedPackages != nil {
		return cachedPackages, nil
	}

	var all []PackageInfo
	for _, repo := range repos {
		resp, err := http.Get(repo)
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		var pkgs []PackageInfo
		json.NewDecoder(resp.Body).Decode(&pkgs)
		all = append(all, pkgs...)
	}

	cachedPackages = all
	return cachedPackages, nil
}

// help
func printHelp() {
	fmt.Println(
		"\n\nCommands:\n" +
			" gopm dwld <pkg> [--ver X]\n" +
			" gopm upd <pkg>\n" +
			" gopm updateall\n" +
			" gopm remove <pkg>\n" +
			" gopm updatepm\n" +
			" gopm list\n" +
			" gopm search <name>\n" +
			" gopm set <repos|check_update> <value>\n" +
			" gopm info\n",
	)
}

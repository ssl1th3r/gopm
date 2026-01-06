package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
)

var defaultRepo = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages/packages.json"
var urllatestjson = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/releases/latest.json"
var checkUpdateFlag = true
var binBase string
var version = "0.0.3.1"
var logo = `            
 _____     _____ _____ 
|   __|___|  _  |     |
|  |  | . |   __| | | |
|_____|___|__|  |_|_|_|
`

type ReleaseInfo struct {
	Version string `json:"version"`
	Binary  string `json:"binary"`
}

type PackageInfo struct {
	Name    string   `json:"name"`
	Version string   `json:"version"`
	Deps    []string `json:"deps,omitempty"`
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
		fmt.Println("Unknown command - usage 'gopm help'")
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
	case "help":
		printHelp()

	default:
		fmt.Println("Unknown command - usage 'gopm help'")
	}
}

func initConfig() {
	usr, err := user.Current()
	if err != nil {
		fmt.Println(RED+"Failed to get user dir:"+RESET, err)
		repos = []string{defaultRepo}
		binBase = defaultRepo[:strings.LastIndex(defaultRepo, "/")]
		checkUpdateFlag = true
		return
	}
	configPath = filepath.Join(usr.HomeDir, ".config", "gopm.conf")
	installedPackagesFile = filepath.Join(usr.HomeDir, ".config", "gopm_installed.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println(BLUE + "Creating default config..." + RESET)
		os.MkdirAll(filepath.Dir(configPath), 0755)
		content := fmt.Sprintf("repos=%s\nbin_base=%s\ncheck_update=true\n", defaultRepo, defaultRepo[:strings.LastIndex(defaultRepo, "/")])
		os.WriteFile(configPath, []byte(content), 0644)
		repos = []string{defaultRepo}
		binBase = defaultRepo[:strings.LastIndex(defaultRepo, "/")]
		checkUpdateFlag = true
		return
	}
	file, err := os.Open(configPath)
	if err != nil {
		fmt.Println(RED+"Failed to open config:"+RESET, err)
		repos = []string{defaultRepo}
		binBase = defaultRepo[:strings.LastIndex(defaultRepo, "/")]
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
			for _, r := range strings.Split(strings.TrimPrefix(line, "repos="), ",") {
				r = strings.TrimSpace(r)
				if r != "" {
					repos = append(repos, r)
				}
			}
		} else if strings.HasPrefix(line, "bin_base=") {
			binBase = strings.TrimSpace(strings.TrimPrefix(line, "bin_base="))
		} else if strings.HasPrefix(line, "check_update=") {
			checkUpdateFlag = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "check_update="))) == "true"
		}
	}
	if len(repos) == 0 {
		repos = []string{defaultRepo}
	}
	if binBase == "" {
		binBase = defaultRepo[:strings.LastIndex(defaultRepo, "/")]
	}
}

func saveConfig() error {
	os.MkdirAll(filepath.Dir(configPath), 0755)
	content := "repos=" + strings.Join(repos, ",") + "\n"
	content += "bin_base=" + binBase + "\n"
	content += "check_update=" + strconv.FormatBool(checkUpdateFlag) + "\n"
	return os.WriteFile(configPath, []byte(content), 0644)
}

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

func checkUpdate() {
	fmt.Println(BLUE + "Checking for gopm updates..." + RESET)
	resp, err := http.Get(urllatestjson)
	if err != nil {
		fmt.Println(YELLOW+"Update check failed:"+RESET, err)
		return
	}
	defer resp.Body.Close()
	var r ReleaseInfo
	if json.NewDecoder(resp.Body).Decode(&r) == nil && r.Version != version {
		fmt.Println(YELLOW+"Update available:"+RESET, r.Version)
	} else {
		fmt.Println(GREEN + "gopm is up to date." + RESET)
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
	fmt.Println(BLUE+"Installing package:"+RESET, name, "version:", ver)
	var pkgMeta *PackageInfo
	pkgs, err := fetchPackages()
	if err != nil {
		fmt.Println(RED + "Failed to fetch packages" + RESET)
		return
	}
	for _, p := range pkgs {
		if p.Name == name {
			pkgMeta = &p
			break
		}
	}
	if pkgMeta != nil && len(pkgMeta.Deps) > 0 {
		fmt.Println(YELLOW+"Dependencies:"+RESET, pkgMeta.Deps)
		for _, dep := range pkgMeta.Deps {
			if !isInstalled(dep) {
				fmt.Println(BLUE+"Installing dependency:"+RESET, dep)
				installPkg(dep, "latest")
			}
		}
	}
	url := fmt.Sprintf("%s/%s/%s/%s", binBase, name, ver, name)
	fmt.Println(BLUE+"Downloading from:"+RESET, url)
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

func isInstalled(name string) bool {
	for _, p := range installedPackages {
		if p.Name == name {
			return true
		}
	}
	return false
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
		cmd := exec.Command("sudo", "cp", tmp, dest)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to install with sudo: %v", err)
		}
		return os.Remove(tmp)
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
	name := args[0]
	if !confirm("Remove " + name + "?") {
		return
	}

	dest := "/usr/bin/" + name
	deleted := false

	if _, err := os.Stat(dest); os.IsNotExist(err) {
		fmt.Println(YELLOW+"File not found, nothing to delete:"+RESET, dest)
	} else {
		if err := os.Remove(dest); err != nil {
			fmt.Println(YELLOW + "Trying with sudo..." + RESET)
			cmd := exec.Command("sudo", "rm", dest)
			cmd.Stdin = os.Stdin
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Println(RED+"Failed to remove binary even with sudo:"+RESET, err)
			} else {
				deleted = true
			}
		} else {
			deleted = true
		}
	}

	removedFromList := false
	for i := 0; i < len(installedPackages); i++ {
		if installedPackages[i].Name == name {
			installedPackages = append(installedPackages[:i], installedPackages[i+1:]...)
			removedFromList = true
			break
		}
	}

	saveInstalledPackages()

	if deleted || removedFromList {
		fmt.Println(GREEN+"Removed:"+RESET, name)
	} else {
		fmt.Println(YELLOW+"Nothing was deleted for package:"+RESET, name)
	}
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
		fmt.Println(RED + "Failed to fetch packages" + RESET)
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
		fmt.Println(BLUE + "Fetching packages from " + repo + RESET)
		resp, err := http.Get(repo)
		if err != nil {
			fmt.Println(YELLOW+"Failed to fetch from "+repo+RESET, err)
			continue
		}
		var pkgs []PackageInfo
		if err := json.NewDecoder(resp.Body).Decode(&pkgs); err != nil {
			fmt.Println(YELLOW+"Failed to decode JSON from "+repo+RESET, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		all = append(all, pkgs...)
	}
	cachedPackages = all
	return cachedPackages, nil
}

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
			" gopm info\n" +
			" \n" +
			" Config path -> ~/.config/gopm.conf\n",
	)
}

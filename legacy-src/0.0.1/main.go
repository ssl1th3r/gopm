package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var urlpackages = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/packages/packages.json"
var urllatestjson = "https://raw.githubusercontent.com/ssl1th3r/gopmpackagesupdate/main/releases/latest.json"

var version = "0.0.1"
var logo = `
  ____       ____  __  __ 
 / ___| ___ |  _ \|  \/  |
| |  _ / _ \| |_) | |\/| |
| |_| | (_) |  __/| |  | |
 \____|\___/|_|   |_|  |_|
`

type ReleaseInfo struct {
	Version string `json:"version"`
	Binary  string `json:"binary"`
}

const (
	RED    = "\033[31m"
	GREEN  = "\033[32m"
	YELLOW = "\033[33m"
	BLUE   = "\033[34m"
	RESET  = "\033[0m"
)

func main() {
	checkUpdate()

	if len(os.Args) < 2 {
		printHelp()
		return
	}

	switch os.Args[1] {
	case "info":
		fmt.Println(logo)
		fmt.Println("Version:", version)
		fmt.Println("Dev: ssl1th3r")

	case "dwld":
		handleDownload(os.Args[2:])

	case "remove":
		removePkg(os.Args[2:])

	case "updatepm":
		updateSelf()

	case "list":
		listAvailable()

	case "help":
		printHelp()

	default:
		fmt.Println(RED + "Unknown command" + RESET)
		printHelp()
	}
}

// update system
func checkUpdate() {
	url := urllatestjson
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var r ReleaseInfo
	if json.NewDecoder(resp.Body).Decode(&r) == nil {
		if r.Version != version {
			fmt.Println(YELLOW + "Update available:" + RESET + " " + r.Version)
		}
	}
}

func updateSelf() {
	url := "https://github.com/ssl1th3r/gopmpackagesupdate/raw/refs/heads/main/bin/gopm"

	fmt.Println(BLUE + "Downloading update..." + RESET)
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println(RED + "Update download failed!" + RESET)
		return
	}
	defer resp.Body.Close()

	tmp := "/tmp/gopm_update"
	out, _ := os.Create(tmp)
	if !printDownloadProgress(resp.Body, out, resp.ContentLength) {
		fmt.Println(RED + "Update download failed!" + RESET)
		return
	}
	out.Close()
	os.Chmod(tmp, 0755)

	fmt.Println(BLUE + "Installing update..." + RESET)
	exec.Command("sudo", "mv", tmp, "/usr/bin/gopm").Run()
	fmt.Println(GREEN + "Updated successfully!" + RESET)
}

//

// Download
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

	downloadPkg(name, ver)
}

func downloadPkg(name, ver string) {
	baseURL := fmt.Sprintf(
		"https://github.com/ssl1th3r/gopmpackagesupdate/raw/main/packages/%s/%s/",
		name, ver,
	)

	tarURL := baseURL + name + "-" + ver + ".tar.gz"
	binURL := baseURL + name

	fmt.Println(BLUE+"Starting download of package:"+RESET, name, ver)

	resp, err := http.Get(tarURL)
	if err == nil && resp.StatusCode == 200 {
		defer resp.Body.Close()

		tmp := "/tmp/" + name + ".tar.gz"
		out, _ := os.Create(tmp)
		if !printDownloadProgress(resp.Body, out, resp.ContentLength) {
			fmt.Println(RED + "Download failed!" + RESET)
			return
		}
		out.Close()

		fmt.Println(BLUE + "Extracting package..." + RESET)
		extractTarGz(tmp, "/tmp")
		os.Remove(tmp)

		installBinary(name, "/tmp/"+name)
		return
	}

	resp, err = http.Get(binURL)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println(RED + "Package not found!" + RESET)
		return
	}
	defer resp.Body.Close()

	tmp := "/tmp/" + name
	out, _ := os.Create(tmp)
	if !printDownloadProgress(resp.Body, out, resp.ContentLength) {
		fmt.Println(RED + "Download failed!" + RESET)
		return
	}
	out.Close()

	installBinary(name, tmp)
}

func installBinary(name, path string) {
	if _, err := os.Stat(path); err != nil {
		fmt.Println(RED + "Binary not found after download!" + RESET)
		return
	}

	fmt.Println(BLUE + "Installing binary..." + RESET)
	os.Chmod(path, 0755)
	err := exec.Command("sudo", "mv", path, "/usr/bin/"+name).Run()
	if err != nil {
		fmt.Println(RED + "Installation failed!" + RESET)
		return
	}

	fmt.Println(GREEN+"Installed successfully:"+RESET, name)
}

//
//Visual download progress

func printDownloadProgress(src io.Reader, dst io.Writer, total int64) bool {
	if total <= 0 {
		_, err := io.Copy(dst, src)
		return err == nil
	}

	buf := make([]byte, 32*1024)
	var downloaded int64
	for {
		n, err := src.Read(buf)
		if n > 0 {
			dst.Write(buf[:n])
			downloaded += int64(n)
			printProgress(downloaded, total)
		}
		if err == io.EOF {
			fmt.Print("\n")
			break
		}
		if err != nil {
			return false
		}
	}
	return true
}

func printProgress(done, total int64) {
	percent := float64(done) / float64(total) * 100
	barLength := 40
	filled := int(percent / 100 * float64(barLength))

	fmt.Printf("\r[")
	for i := 0; i < filled; i++ {
		fmt.Print("=")
	}
	for i := filled; i < barLength; i++ {
		fmt.Print(" ")
	}
	fmt.Printf("] %.1f%%", percent)
}

// Remove pkg
func removePkg(args []string) {
	if len(args) < 1 {
		fmt.Println("Usage: gopm remove <pkg>")
		return
	}
	pkg := args[0]
	exec.Command("sudo", "rm", "-f", "/usr/bin/"+pkg).Run()
	fmt.Println(GREEN+"Removed:"+RESET, pkg)
}

// Extract Tar
func extractTarGz(src, dest string) {
	file, _ := os.Open(src)
	defer file.Close()

	gzr, _ := gzip.NewReader(file)
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if h.Typeflag == tar.TypeReg {
			path := filepath.Join(dest, h.Name)
			f, _ := os.Create(path)
			io.Copy(f, tr)
			f.Close()
		}
	}
}

//

// Check version
func resolveVersion(pkg, cond string) string {
	url := urlpackages
	resp, err := http.Get(url)
	if err != nil {
		return "latest"
	}
	defer resp.Body.Close()

	var list []map[string]string
	json.NewDecoder(resp.Body).Decode(&list)

	best := "0.0.0"
	base := strings.Trim(cond, ">=")

	for _, p := range list {
		if p["name"] == pkg && compare(p["version"], base) {
			best = p["version"]
		}
	}
	return best
}

// Compare
func compare(a, b string) bool {
	as := strings.Split(a, ".")
	bs := strings.Split(b, ".")
	for i := 0; i < len(as); i++ {
		ai, _ := strconv.Atoi(as[i])
		bi, _ := strconv.Atoi(bs[i])
		if ai > bi {
			return true
		}
		if ai < bi {
			return false
		}
	}
	return true
}

// List pkg
func listAvailable() {
	url := urlpackages
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var pkgs []map[string]string
	json.NewDecoder(resp.Body).Decode(&pkgs)

	for _, p := range pkgs {
		fmt.Printf("%s%s%s (%s)\n", GREEN, p["name"], RESET, p["version"])
	}
}

// Help
func printHelp() {
	fmt.Println(
		"GoPM\n\n" +
			"Commands:\n" +
			" gopm dwld <pkg> [--ver X]   Download and install\n" +
			" gopm remove <pkg>           Remove package\n" +
			" gopm updatepm               Update gopm\n" +
			" gopm list                   List packages\n" +
			" gopm info                   Info\n",
	)
}

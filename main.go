package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "strings"
    "sort"

    "gopkg.in/alecthomas/kingpin.v1"
    "github.com/kballard/go-shellquote"
)

var (
    our_dir string = fmt.Sprintf("%s/.cache/dmenu_hist", os.Getenv("HOME"))
    history_path string = our_dir + "/history"
    arg_noop = kingpin.Flag("noop", "Do all except executing dmenu itself, for timing.").Bool()
    arg_edit = kingpin.Flag("edit", fmt.Sprintf("Open gvim with history file (%s)", history_path)).Bool()
    arg_verbose = kingpin.Flag("verbose", "Be more verbose (show some debug info)").Bool()
    extra_cmd = map[string]func(){"!edit-history": LaunchEditor}
)

func _err(e error) {
    if e != nil {
        fmt.Println("ERROR: ", e)
        os.Exit(1)
    }
}

func debug(msgs ...interface{}) {
    if *arg_verbose {
        fmt.Println(msgs...)
    }
}

func IsExec(file os.FileInfo) bool {
    return (file.Mode() & 0111) != 0
}

func In(what string, where []string) bool {
    for _, s := range where {
        if s == what { return true; }
    }
    return false
}

func InExtra(what string) bool {
    for key := range extra_cmd {
        if key == what { return true; }
    }
    return false
}

func ScanPathsFlat() (app_names map[string]bool) {
    app_names = make(map[string]bool)

    paths := strings.Split(os.Getenv("PATH"), ":")
    for _, dir_path := range paths {
        directory, err := os.Open(dir_path)
        _err(err)
        files, err := directory.Readdir(0)
        _err(err)

        debug("path:", dir_path, len(files))

        for _, file := range files {
            if ! file.IsDir() && IsExec(file) {
                app_names[file.Name()] = true
            }
        }
    }
    return app_names
}

func LoadHistory(history_path string) []string {
    debug("loading:", history_path)
    file, err := os.OpenFile(history_path, os.O_CREATE | os.O_RDONLY, 0644)
    _err(err)
    defer file.Close()

    data, err := ioutil.ReadAll(file)
    _err(err)

    lines_rev := strings.Split(strings.TrimSpace(string(data)), "\n")
    end := len(lines_rev) + len(extra_cmd)
    lines := make([]string, end)

    for cmd := range extra_cmd {
        end--
        lines[end] = cmd
    }
    for _, line := range lines_rev {
        line = strings.TrimSpace(line)
        if line != "" && !In(line, lines) && !InExtra(line) {
            end--
            lines[end] = line
        }
    }

    return lines[end:]
}

func SaveHistory(history_path string, history []string, last_used string) {
    debug("adding:", last_used, "to:", history_path)
    file, err := os.OpenFile(history_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
    _err(err)
    defer file.Close()

    totalbytes := 0
    bytes := 0
    for _, choice := range history {
        if choice != last_used {
            bytes, err = file.WriteString(choice+"\n")
            totalbytes += bytes
        }
    }
    bytes, err = file.WriteString(last_used+"\n")
    _err(err)
    debug("bytes written:", totalbytes+bytes)
}

func LaunchEditor() {
    //editor := os.Getenv("EDITOR")
    //if editor == "" { editor = "/usr/bin/vi" }
    //cmd := exec.Command("xterm", "-e", editor, history_path)
    cmd := exec.Command("gvim", history_path)
    err := cmd.Start()
    _err(err)
    os.Exit(0)
}

func main() {
    kingpin.Parse()

    if *arg_edit { LaunchEditor() }

    history := LoadHistory(history_path)
    app_hash := ScanPathsFlat()

    // remove those we have in history
    for _, used := range history { delete(app_hash, used) }
    // convert to list of names
    app_names := make([]string, len(app_hash))
    app_idx := 0
    for app := range app_hash { app_names[app_idx] = app; app_idx++ }
    // and sort the list
    sort.Strings(app_names)


    debug("history:", strings.Join(history, " "))
    debug("apps count:", len(app_names))

    if *arg_noop {
        os.Exit(0)
    }

    dmenu := exec.Command("dmenu")
    dmenu_in, err := dmenu.StdinPipe()
    _err(err)
    dmenu_out, err := dmenu.StdoutPipe()
    _err(err)

    err = dmenu.Start()
    _err(err)

    for _, app := range (history) { dmenu_in.Write([]byte(app + "\n")) }
    for _, app := range (app_names) { dmenu_in.Write([]byte(app + "\n")) }
    dmenu_in.Close()

    choice_bytes, err := ioutil.ReadAll(dmenu_out)
    dmenu.Wait()

    choice := strings.TrimSpace(string(choice_bytes))
    if choice == "" { os.Exit(0) }

    for cmd, action := range extra_cmd {
        if choice == cmd {
            action()
        }
    }

    choice_parts, err := shellquote.Split(choice)
    _err(err)

    prog := choice_parts[0]
    args := choice_parts[1:]

    found, err := exec.LookPath(prog)
    _err(err)

    SaveHistory(history_path, history, choice)
    cmd := exec.Command(found, args...)
    err = cmd.Start()
    _err(err)
}

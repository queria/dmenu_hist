package main

/******
* Copyright © 2015, Queria Sa-Tas <public@sa-tas.net> All rights reserved.
*
* Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:
*
*   * Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
*
*   * Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
*
*   * Neither the name of the Queria Sa-Tas nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.
*
* THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS “AS IS” AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL QUERIA SA-TAS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING INCLUDING ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
*/

import (
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "strconv"
    "strings"
    "sort"
    "time"

    "gopkg.in/alecthomas/kingpin.v1"
    "github.com/kballard/go-shellquote"
    "github.com/queria/golang-go-xdg"
)

var (
    g_history_path string
    g_cache_path string
    g_extra_cmd = map[string]func(){"!edit-history": LaunchEditor}
    arg_noop = kingpin.Flag("noop", "Do all except executing dmenu itself, for timing.").Bool()
    arg_edit = kingpin.Flag("edit", fmt.Sprintf("Open gvim with history file (%s)", g_history_path)).Bool()
    arg_verbose = kingpin.Flag("verbose", "Be more verbose (show some debug info)").Bool()
)

func init() {
    g_history_path, _ = xdg.Data.Ensure("dmenu_hist/history")
    g_cache_path, _= xdg.Cache.Ensure("dmenu_hist/app_cache")
}

func _err(e error) {
    if e != nil {
        fmt.Println("ERROR: ", e)
        os.Exit(1)
    }
}

func debug(msgs ...interface{}) {
    if *arg_verbose {
        fmt.Fprintf(os.Stderr, "[debug] %v\n", msgs)
    }
}

func timeit(label string, started time.Time) {
    if *arg_verbose {
        fmt.Fprintln(os.Stderr, "[timeit]", time.Since(started), label)
    }
}

func IsExec(file os.FileInfo) bool {
    return (file.Mode() & 0111) != 0
}

func In(what string, where []string) bool {
    return IndexOf(what, where) >= 0
}

func IndexOf(what string, where []string) int {
    for idx, s := range where {
        if s == what { return idx; }
    }
    return -1
}

func InExtra(what string) bool {
    for key := range g_extra_cmd {
        if key == what { return true; }
    }
    return false
}

func ScanPaths() (app_names []string) {
    defer timeit("scanning path, converting and sorting app names", time.Now())

    // collect app names in map to eliminate duplicates
    app_hash := make(map[string]bool)

    // go over all directories in path
    // and find executable files in there
    paths := strings.Split(os.Getenv("PATH"), ":")
    for _, dir_path := range paths {
        directory, err := os.Open(dir_path)
        _err(err)
        files, err := directory.Readdir(0)
        _err(err)

        debug("path:", dir_path, len(files))

        for _, file := range files {
            if ! file.IsDir() && IsExec(file) {
                app_hash[file.Name()] = true
            }
        }
    }

    defer timeit("converting and sorting app names", time.Now())
    // convert to list of names
    app_names = make([]string, len(app_hash))
    app_idx := 0
    for app := range app_hash { app_names[app_idx] = app; app_idx++ }

    defer timeit("sorting app names", time.Now())
    // and sort the list
    sort.Strings(app_names)
    return app_names
}

func PathLastChangedAt() time.Time {
    var max_time time.Time
    paths := strings.Split(os.Getenv("PATH"), ":")
    for _, dir_path := range paths {
        info, err := os.Stat(dir_path)
        if err == nil && (info.ModTime().After(max_time)) {
            max_time = info.ModTime()
        }
     }
     return max_time
}

func ReadLines(file_path string) (lines []string) {
    defer timeit("reading lines from: "+file_path, time.Now())
    file, err := os.OpenFile(file_path, os.O_CREATE | os.O_RDONLY, 0644)
    _err(err)
    defer file.Close()

    data, err := ioutil.ReadAll(file)
    _err(err)

    lines_raw := strings.Split(strings.TrimSpace(string(data)), "\n")
    for _, line := range lines_raw {
        line = strings.TrimSpace(line)
        if line == "" { continue; }
        lines = append(lines, line)
    }
    return lines
}

func WriteLines(file_path string, lines []string) (bytes_written int) {
    file, err := os.OpenFile(file_path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
    _err(err)
    defer file.Close()

    bytes_written = 0
    bytes_per_line := 0
    for _, line := range lines {
        bytes_per_line, err = file.WriteString(line+"\n")
        _err(err)
        bytes_written += bytes_per_line
    }
    debug("written out", bytes_written, "bytes into", file_path)
    return bytes_written
}

func LoadCache(path string, newer_than time.Time) (cached_apps []string) {
    defer timeit("trying to load cache: "+path, time.Now())
    info, err := os.Stat(path)
    if err != nil || info.ModTime().Before(newer_than) {
        return nil
    }

    return ReadLines(path)
}

func SaveCache(path string, app_names []string) {
    defer timeit("saving of cache: "+path, time.Now())
    WriteLines(path, app_names)
}

func LoadOrScanPaths() (app_names []string) {
    paths_changed_at := PathLastChangedAt()
    app_names = LoadCache(g_cache_path, paths_changed_at)

    if app_names == nil || len(app_names) == 0 {
        app_names = ScanPaths()
        SaveCache(g_cache_path, app_names)
    }

    return app_names
}

type UsedApp struct {
    Cmd string
    Count int
}
func (a UsedApp) String() string { return fmt.Sprintf("%v:%v", a.Cmd, a.Count) }
type MostUsed []UsedApp
func (apps MostUsed) Len() int { return len(apps) }
func (apps MostUsed) Swap(i, j int) { apps[i], apps[j] = apps[j], apps[i] }
func (apps MostUsed) Less(i, j int) bool {
    // less (first) is the one with bigger usage count
    return apps[i].Count > apps[j].Count
}

func SplitHistoryLine(line string) (app UsedApp) {
    delim := strings.Index(line, ":")
    if delim == -1 {
        // backward compat - just names without count
        app.Cmd = line
        app.Count = 1
        return
    }

    app.Cmd = line[0:delim]
    count, err := strconv.Atoi(
        line[delim+1 : len(line)])
    _err(err)
    app.Count = count
    return
}

func LoadHistory(history_path string) []UsedApp {
    defer timeit("loading history from: " + history_path, time.Now())

    lines_raw := ReadLines(history_path)

    end := len(lines_raw) + len(g_extra_cmd)
    history := make([]UsedApp, 0, end)

    for _, line := range lines_raw {
        app := SplitHistoryLine(line)
        debug("read history line as:", line, app)
        if app.Cmd == "" || InExtra(app.Cmd) { continue; }

        history = append(history, app)
    }
    // internal commands always at the end of list
    for cmd := range g_extra_cmd {
        history = append(history, UsedApp{Cmd: cmd, Count: 0})
    }

    return history
}

func SaveHistory(history_path string, history []UsedApp, last_cmd string) {
    defer timeit(
        fmt.Sprintf("saving history: %v with: %v entries", history_path, len(history)),
        time.Now())

    clean_history := make([]string, 0, len(history) + 1)
    for _, app := range(history) {
        if strings.HasPrefix(app.Cmd, "!") { continue; } // exclude internal commands from history
        if last_cmd != "" && last_cmd == app.Cmd {
            app.Count += 1
            last_cmd = ""
        }
        clean_history = append(clean_history, app.String())
    }
    if last_cmd != "" && !strings.HasPrefix(last_cmd, "!") {
        clean_history = append(clean_history, UsedApp{Cmd: last_cmd, Count: 1}.String())
    }

    debug("saving history:", clean_history)

    WriteLines(history_path, clean_history)
}

func FilterOutHistory(app_names []string, history []UsedApp) (filtered_names []string) {
    defer timeit("filtering out history from app names", time.Now())
    for _, used := range history {
        debug("trying to filter out:", used, "in", len(app_names))
        idx := sort.SearchStrings(app_names, used.Cmd)
        if app_names[idx] == used.Cmd {
            app_names = append(app_names[:idx], app_names[idx+1:]...)
        }
    }
    return app_names
}

func LaunchEditor() {
    //editor := os.Getenv("EDITOR")
    //if editor == "" { editor = "/usr/bin/vi" }
    //cmd := exec.Command("xterm", "-e", editor, g_history_path)
    cmd := exec.Command("gvim", g_history_path)
    err := cmd.Start()
    _err(err)
    os.Exit(0)
}

func stripExtraArgs(args []string) (normal []string, extra []string) {
    delim := IndexOf("--", args)
    if delim >= 0 {
        normal = args[0:delim]
        extra = args[delim+1:len(args)]
    } else {
        normal = args
    }
    return
}

func main() {
    defer timeit("main", time.Now())
    args, dmenu_opts := stripExtraArgs(os.Args[1:])
    kingpin.MustParse(kingpin.CommandLine.Parse(args))

    debug("remaining opts:", dmenu_opts)

    if *arg_edit { LaunchEditor() }

    history := LoadHistory(g_history_path)
    sort.Sort(MostUsed(history))

    app_names := LoadOrScanPaths()


    debug("before filter", len(app_names))
    app_names = FilterOutHistory(app_names, history)


    debug("history:", history)
    debug("apps count:", len(app_names))

    if *arg_verbose {
        for _, app := range (history) { os.Stdout.Write([]byte(app.Cmd + "\n")) }
        for _, app := range (app_names) { os.Stdout.Write([]byte(app + "\n")) }
    }


    if *arg_noop {
        return
    }

    dmenu := exec.Command("dmenu", dmenu_opts...)
    dmenu_in, err := dmenu.StdinPipe()
    _err(err)
    dmenu_out, err := dmenu.StdoutPipe()
    _err(err)

    err = dmenu.Start()
    _err(err)

    for _, app := range (history) { dmenu_in.Write([]byte(app.Cmd + "\n")) }
    for _, app := range (app_names) { dmenu_in.Write([]byte(app + "\n")) }
    dmenu_in.Close()

    choice_bytes, err := ioutil.ReadAll(dmenu_out)
    dmenu.Wait()

    choice := strings.TrimSpace(string(choice_bytes))
    if choice == "" { os.Exit(0) }

    for cmd, action := range g_extra_cmd {
        if choice == cmd {
            action()
        }
    }

    choice_parts, err := shellquote.Split(choice)
    _err(err)

    prog := choice_parts[0]
    prog_args := choice_parts[1:]

    found, err := exec.LookPath(prog)
    _err(err)

    SaveHistory(g_history_path, history, choice)
    cmd := exec.Command(found, prog_args...)
    err = cmd.Start()
    _err(err)
}

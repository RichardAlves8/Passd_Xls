package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	stdlog "log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Info struct {
	Folder   string
	Ext      string
	Files    int
	Modified string
	hasPass  int
}

func main() {
	if len(os.Args) < 2 {
		stdlog.Fatal("Uso: programa <path1> [path2] [path3] ...")
	}

	loadEnv(".env")
	initCredentials()

	if err := os.MkdirAll("Log", 0755); err != nil {
		stdlog.Fatal("Erro ao criar diretório Log:", err)
	}
	if err := os.MkdirAll("Csv", 0755); err != nil {
		stdlog.Fatal("Erro ao criar diretório Csv:", err)
	}

	logFile, err := os.OpenFile(filepath.Join("Log", "OLE.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		stdlog.Fatal("Erro ao abrir log:", err)
	}
	defer logFile.Close()
	oleLogger = stdlog.New(logFile, "", 0)

	roots := os.Args[1:]
	re := regexp.MustCompile(`(?i)\.xls.?$`)
	nums := 0

	var arquivos []string
	for _, root := range roots {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}
			if re.MatchString(info.Name()) {
				nums++
				fmt.Printf("\rTotal: %v", nums)
				arquivos = append(arquivos, path)
			}
			return nil
		})
	}

	type resultado struct {
		path    string
		hasPass bool
		modTime time.Time
	}

	results := make([]resultado, len(arquivos))
	var wg sync.WaitGroup
	var mu sync.Mutex
	sem := make(chan struct{}, 8) // @CHANGE WORKERS
	processed := 0
	totalFiles := len(arquivos)

	for i, arquivo := range arquivos {
		wg.Add(1)
		go func(idx int, a string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			info, err := os.Stat(a)
			modTime := time.Time{}
			if err == nil {
				modTime = info.ModTime()
			}
			hasPass := changeOLE(a, true)
			results[idx] = resultado{path: a, hasPass: hasPass, modTime: modTime}

			mu.Lock()
			processed++
			fmt.Printf("\rProcessando: %d / %d", processed, totalFiles)
			mu.Unlock()
		}(i, arquivo)
	}

	wg.Wait()
	fmt.Println()

	extMap := map[string]*Info{}
	total, totalpass := 0, 0

	for _, r := range results {
		total++
		if r.hasPass {
			fmt.Printf("\n(%v): %v", total, r.path)
		}

		ext := strings.ToLower(filepath.Ext(r.path))
		dir := filepath.Dir(r.path)
		key := dir + "|" + ext

		if _, exists := extMap[key]; !exists {
			extMap[key] = &Info{Folder: dir, Ext: ext}
		}

		entry := extMap[key]
		entry.Files++

		if entry.Modified == "" {
			entry.Modified = r.modTime.Format("02/01/2006")
		} else {
			saved, _ := time.Parse("02/01/2006", entry.Modified)
			if r.modTime.After(saved) {
				entry.Modified = r.modTime.Format("02/01/2006")
			}
		}

		if r.hasPass {
			entry.hasPass++
			totalpass++
		}
	}

	outputFile, err := os.Create(filepath.Join("Csv", "relat.csv"))
	if err != nil {
		stdlog.Fatal("Erro ao criar relat.csv:", err)
	}
	defer outputFile.Close()

	outputFile.WriteString("\xEF\xBB\xBF") // BOM UTF-8
	writer := csv.NewWriter(outputFile)
	defer writer.Flush()

	writer.Write([]string{"Montagem", "Raiz", "Diretorio", "Extensao", "Arquivos", "Data_Alteracao", "Com_senha"})

	for _, v := range extMap {
		volumeName := filepath.VolumeName(v.Folder)
		withoutVolume := strings.TrimPrefix(v.Folder, volumeName)
		withoutVolume = strings.TrimPrefix(withoutVolume, string(filepath.Separator))

		parts := strings.SplitN(withoutVolume, string(filepath.Separator), 2)
		raiz, diretorioRestante := "", ""
		if len(parts) > 0 {
			raiz = parts[0]
		}
		if len(parts) > 1 {
			diretorioRestante = parts[1]
		}

		writer.Write([]string{
			volumeName,
			raiz,
			diretorioRestante,
			v.Ext,
			strconv.Itoa(v.Files),
			v.Modified,
			strconv.Itoa(v.hasPass),
		})
	}

	fmt.Println("\nSucess!")
	register(totalpass, total, extMap, filepath.Join("Csv", "register.csv"))
}

func register(totalpass, total int, extMap map[string]*Info, reg string) {
	file, err := os.OpenFile(reg, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		stdlog.Fatal(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	now := time.Now().Format("02/01/2006")

	// linha de totais globais
	writer.Write([]string{
		now,
		strconv.Itoa(totalpass),
		strconv.Itoa(total),
		"all",
		"",
		".*",
	})

	type chave struct{ montagem, raiz, ext string }
	type acum struct{ files, hasPass int }

	acumMap := map[chave]*acum{}

	for _, v := range extMap {
		volumeName := filepath.VolumeName(v.Folder)
		withoutVolume := strings.TrimPrefix(v.Folder, volumeName)
		withoutVolume = strings.TrimPrefix(withoutVolume, string(filepath.Separator))

		parts := strings.SplitN(withoutVolume, string(filepath.Separator), 2)
		raiz := ""
		if len(parts) > 0 {
			raiz = parts[0]
		}

		k := chave{montagem: volumeName, raiz: raiz, ext: v.Ext}
		if _, ok := acumMap[k]; !ok {
			acumMap[k] = &acum{}
		}
		acumMap[k].files += v.Files
		acumMap[k].hasPass += v.hasPass
	}

	for k, a := range acumMap {
		writer.Write([]string{
			now,
			strconv.Itoa(a.hasPass),
			strconv.Itoa(a.files),
			k.montagem,
			k.raiz,
			k.ext,
		})
	}
}

func loadEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return // .env é opcional
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

package main

import (
	"archive/zip"
	"bytes"
	"io"
	stdlog "log"
	"os"
	"regexp"
	"time"
)

var (
	reUID *regexp.Regexp
	rePWD *regexp.Regexp

	newUID, newPWD string

	connectionsXML = "xl/connections.xml" //!!!
	libRegEx, _    = regexp.Compile(`(?i)\.xls[mx]$`)

	oleLogger *stdlog.Logger
)

func initCredentials() {
	oldUID := os.Getenv("OLD_UID")
	oldPWD := os.Getenv("OLD_PWD")
	newUID = os.Getenv("NEW_UID")
	newPWD = os.Getenv("NEW_PWD")

	if oldUID == "" || oldPWD == "" || newUID == "" || newPWD == "" {
		stdlog.Fatal("Configurações de ambientes não encontradas(.env ou var).")
	}

	reUID = regexp.MustCompile(`(?i)UID=` + regexp.QuoteMeta(oldUID))
	rePWD = regexp.MustCompile(`(?i)PWD=` + regexp.QuoteMeta(oldPWD))
}

func changeOLE(arquivo string, readonly bool) bool {
	if !libRegEx.MatchString(arquivo) {
		return false
	}

	planilhaZip, err := zip.OpenReader(arquivo)
	if err != nil {
		oleLogger.Printf("[Error on read | %v]: %v\n", nowtime(), arquivo)
		return false
	}
	defer planilhaZip.Close()

	var modifiedText string
	var foundTarget, needsChange bool

	for _, xmlFile := range planilhaZip.File {
		if xmlFile.Name != connectionsXML {
			continue
		}

		foundTarget = true

		openXML, err := xmlFile.Open()
		if err != nil {
			oleLogger.Printf("[Error on open | %v]: %v\n", nowtime(), arquivo)
			return false
		}

		ioXml, err := io.ReadAll(openXML)
		openXML.Close()
		if err != nil {
			oleLogger.Printf("[Error on read | %v]: %v\n", nowtime(), arquivo)
			return false
		}

		stringXml := string(ioXml)

		if reUID.MatchString(stringXml) {
			needsChange = true
			stringXml = reUID.ReplaceAllString(stringXml, "UID="+newUID)
			stringXml = rePWD.ReplaceAllString(stringXml, "PWD="+newPWD)

			if readonly {
				return true
			}
			//Processo Inicia em prol d alteração das credencias
			oleLogger.Printf("[Match Regex | %v]: %v\n", nowtime(), arquivo)
		}

		modifiedText = stringXml
		break
	}

	if !foundTarget || !needsChange {
		return false
	}

	//por precaução usamos arquivos temporários
	tmpFile := arquivo + ".tmp"
	tmpXls, err := os.Create(tmpFile)

	if err != nil {
		oleLogger.Printf("[Err tmp file | %v]: %v\n", nowtime(), tmpFile)
		return false
	}

	zipWriterTmp := zip.NewWriter(tmpXls)

	/*
			o processo de extração visa buscar o match do Regex no arquivo xmlConnections.xlm
		1 — Passa arquivo por arquivo dentro do zip (Eles funcionam em cache automaticamente!)
	*/
	for _, xmlFile := range planilhaZip.File {
		if xmlFile.Name == connectionsXML {
			targetFileConfig, err := zipWriterTmp.Create(connectionsXML)
			if err != nil {
				oleLogger.Printf("[Error create xml in zip | %v]: %v\n", nowtime(), arquivo)
				zipWriterTmp.Close()
				tmpXls.Close()

				os.Remove(tmpFile)
				return false
			}
			//cria cópia do arquivo de conf original
			io.Copy(targetFileConfig, bytes.NewBufferString(modifiedText))
			continue
		}

		//inicio do processamento do arquivo xml
		openXML, err := xmlFile.Open()
		if err != nil {
			oleLogger.Printf("[Error open xml in loop | %v]: %v - %v\n", nowtime(), arquivo, xmlFile.Name)
			zipWriterTmp.Close()
			tmpXls.Close()
			os.Remove(tmpFile)
			return false
		}

		xmlTempFile, err := zipWriterTmp.Create(xmlFile.Name)
		if err != nil {
			openXML.Close()
			oleLogger.Printf("[Error create file in zip | %v]: %v\n", nowtime(), arquivo)
			zipWriterTmp.Close()
			tmpXls.Close()
			os.Remove(tmpFile)
			return false
		}

		io.Copy(xmlTempFile, openXML)
		openXML.Close()
	}

	zipWriterTmp.Close()
	tmpXls.Close()
	planilhaZip.Close()

	if err = os.Remove(arquivo); err != nil {
		oleLogger.Printf("[Error on del file | %v]: %v\n%v\n", nowtime(), arquivo, err)
		return false
	}

	if err = os.Rename(tmpFile, arquivo); err != nil {
		oleLogger.Printf("[Error on rename | %v]: %v\n%v\n", nowtime(), arquivo, err)
		return false
	}

	oleLogger.Printf("[Ok | %v]: %v\n", nowtime(), arquivo)
	return true
}

func nowtime() string {
	return time.Now().Format("02/01/2006, 15:04")
}

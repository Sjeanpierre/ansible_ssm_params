package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

//When using binary modules Ansible creates a json file containing all the arguments for the module to use
//The only argument passed from Ansible will be the filepath
type ParamArgs struct {
	Region     string
	Group      string
	Version    string
	Parameters map[string]string
}

//Ansible requires responses to be returned in JSON format with the following details
//Message - text to return to Ansible
//Changed - bool value indicating if any changes were made
//Failed - bool value indicating failure within the module
type responseStruct struct {
	Msg     string `json:"msg"`
	Changed bool   `json:"changed"`
	Failed  bool   `json:"failed"`
}

func exitJSON(responseBody responseStruct) {
	returnResponse(responseBody)
}

func failJSON(responseBody responseStruct) {
	responseBody.Failed = true
	returnResponse(responseBody)
}

func returnResponse(responseBody responseStruct) {
	var response []byte
	var err error
	var e error
	response, err = json.Marshal(responseBody)
	if err != nil {
		response, e = json.Marshal(responseStruct{Msg: "Invalid response object"})
		if e != nil {
			log.Fatalln("Unhandled exception in json marshalling")
		}
	}
	fmt.Println(string(response))
	//if failed state is present, os exit 1 will be used to indicated failure of command
	if responseBody.Failed {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}

func main() {
	f, err := os.OpenFile("./pusher.log", os.O_RDWR | os.O_CREATE | os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	log.Println("Starting module run")

	var resp responseStruct
	//Check length of args passed into go
	//The first value in this slice is the path to the program
	//The second value is the path at which Ansible has stored json params file (argsFile)
	if len(os.Args) != 2 {
		resp.Msg = "No argument file provided"
		failJSON(resp)
	}

	argsFile := os.Args[1]

	text, err := ioutil.ReadFile(argsFile)
	if err != nil {
		resp.Msg = "Could not read configuration file: " + argsFile
		failJSON(resp)
	}

	var moduleArgs ParamArgs
	err = json.Unmarshal(text, &moduleArgs)
	if err != nil {
		resp.Msg = "Configuration file not valid JSON: " + argsFile
		failJSON(resp)
	}
	//todo,validate body
	log.Printf("Received params: %+v", moduleArgs)
	results := moduleArgs.push()
	if len(results["failed"]) > 0 {
		resp.Msg = fmt.Sprintf("Failed to write all params:" +
			" Pushed = %+v  Skipped = %v  Failed = %v",
			results["pushed"],
			results["skipped"],
			results["failed"])
		failJSON(resp)
		return
	}

	if len(results["pushed"]) == 0 {
		resp.Changed = false
		resp.Msg = fmt.Sprintf("No params were written:" +
			" Pushed = %+v  Skipped = %v  Failed = %v",
			results["pushed"],
			results["skipped"],
			results["failed"])
		exitJSON(resp)
		return
	}
	resp.Msg = fmt.Sprintf("Processed Params: Pushed = %+v  Skipped = %v  Failed = %v",
		results["pushed"],
		results["skipped"],
		results["failed"])

	resp.Changed = true

	exitJSON(resp)
}
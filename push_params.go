package main

import (
	"github.com/aws/aws-sdk-go/service/ssm"
	"github.com/aws/aws-sdk-go/aws"
	"fmt"
	"crypto/sha256"
	"strings"
	"log"
	"encoding/json"
	"encoding/base64"
	"github.com/pkg/errors"
	"bytes"
	"compress/gzip"
)

var (
	HistoryCache = make(map[string]bool)
)



func (p ParamArgs) cacheKey() string {
	id := fmt.Sprintf("%s@%s", p.Group,p.Version)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(id)))
}

func mapExisting(c ssmClient, group string) {
	paramNames := c.WithPrefix(group)
	params := paramNames.IncludeHistory(c)
	for _, param := range params {
		for _,v := range param.Versions {
			log.Printf("adding %s@%s to map",param.Name,v.Version)
			HistoryCache[v.checksum(param.Name)] = true
		}
	}
}

func checksum(id string) string{
	return fmt.Sprintf("%x", sha256.Sum256([]byte(id)))
}

func (p *ParamArgs) serialize() (string,error){
	//gzip -9 | base64
	params := p.Parameters
	//Create JSON representation of struct
	paramsJson, err := json.Marshal(params)
	if err != nil {
		e := errors.New("Counld not marshall params to JSON")
		return "",e
	}
	var buf bytes.Buffer
	//Compress JSON
	gz, err := gzip.NewWriterLevel(&buf,9)
	if err != nil {
		e := errors.New("Counld not initiate gzip")
		return "",e
	}
	_, err = gz.Write(paramsJson)
	gz.Flush()
	gz.Close()
	if err != nil {
		e := errors.New("Could not produce JSON string from params")
		return "",e
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	log.Println(encoded)


	return encoded,nil
}

func (p ParamArgs) push() map[string][]string{
	c := ssmClient{NewClient(p.Region)}
	mapExisting(c,p.Group)
	var pushed, skipped, failed []string
	log.Println("Checking values of single key")
	log.Println(p.SingleKey)
	if p.SingleKey == true {
		serialzedParams,err := p.serialize()
		if err != nil {
			log.Println("Error: Could not serialize params")

		}
		//todo check length of serialized string
		mp := make(map[string]string)
		mp[p.Version] = serialzedParams
		p.Parameters = mp
	}
	for name, value := range p.Parameters {
		n := strings.Join([]string{p.Group,name},".")
		id := fmt.Sprintf("%s.%s@%s",n,value,p.Version)
		uid := checksum(id)
		if HistoryCache[uid] {
			log.Printf("[Duplicate] - skipping param %s at " +
				"version %s with value hash of %s",n,p.Version,checksum(value))
			skipped = append(skipped,n)
			continue
		}
		input := &ssm.PutParameterInput{
			Description: aws.String(p.Version),
			Name: aws.String(n),
			Overwrite: aws.Bool(true),
			Type: aws.String("SecureString"),
			Value: aws.String(value),
		}
		log.Printf("[Writting] - param %s at " +
			"version %s with value hash of %s",n,p.Version,checksum(value))
		pushed = append(pushed,n)
		_, err := c.client.PutParameter(input)
		if err != nil {
			msg := fmt.Sprintf("%s-%s",n,err.Error())
			failed = append(failed,msg)
		}
	}
	return map[string][]string{"pushed":pushed,"skipped":skipped,"failed":failed}
}
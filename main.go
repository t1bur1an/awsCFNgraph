package main

import (
	"bytes"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/goccy/go-graphviz"
	"github.com/goccy/go-graphviz/cgraph"
	"log"
	"strings"
	"sync"
	"time"
)

type Export struct {
	stackId string
	name string
}

type Connection struct {
	sourceStack string
	exportName string
	destinationStack string
}

func main() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	credos, err := sess.Config.Credentials.Get()
	fmt.Println(err)
	fmt.Println(credos)
	svc := cloudformation.New(sess)

	graphBytes, err := renderDOTGraph()
	if err != nil {
		panic(err)
	}
	graph, err := graphviz.ParseBytes(graphBytes)
	if err != nil {
		panic(err)
	}

	var exportsList []Export
	var nextToken *string

	nodeMap := make(map[string]*cgraph.Node)

	for {
		result, err := svc.ListExports(
			&cloudformation.ListExportsInput{
				NextToken: nextToken,
			},
		)

		if err != nil {
			fmt.Println(err.Error())
			return
		}
		for _, export := range result.Exports {
			stackName := strings.Split(*export.ExportingStackId, "/")[1]
				graphNode, err := graph.CreateNode(stackName)
				if err != nil {
					panic(err)
				}
				nodeMap[stackName] = graphNode

				exportsList = append(
					exportsList,
					Export{
						stackId: stackName,
						name: *export.Name,
					},
				)
		}
		nextToken = result.NextToken
		if nextToken == nil {
			break
		}
	}

	var withoutImports []string

	withoutImportsQueue := make(chan string, len(exportsList))
	connectionsQueue := make(chan Connection, len(exportsList)*100)

	var wg sync.WaitGroup
	wg.Add(len(exportsList))
	for _, export := range exportsList {
		go func(export Export, queue chan string, wg *sync.WaitGroup, connections chan Connection) {
			defer wg.Done()
			doneFlag := false
			for {
				result, err := svc.ListImports(
					&cloudformation.ListImportsInput{
						ExportName: aws.String(export.name),
					},
				)

				if err != nil {
					if awsErr, ok := err.(awserr.Error); ok {
						if awsErr.Code() == "ValidationError" {
							queue <- export.name
							doneFlag = true
						} else if awsErr.Code() == "Throttling" {
							time.Sleep(1 * time.Second)
						} else if awsErr.Code() == "RequestError" {
							time.Sleep(1 * time.Second)
						} else {
							panic(err)
						}
					} else {
						panic(err)
					}
				} else {
					if result.Imports != nil {
						for _, stack := range result.Imports {
							connectionsQueue <- Connection{
								sourceStack: export.stackId,
								destinationStack: *stack,
								exportName: export.name,
							}
						}
					}
					doneFlag = true
					fmt.Println(export.stackId, export.name, result)
				}

				if doneFlag == true {
					break
				}
			}
		}(export, withoutImportsQueue, &wg, connectionsQueue)
	}
	wg.Wait()
	fmt.Println("goroutines done")
	close(withoutImportsQueue)
	close(connectionsQueue)
	fmt.Println("chains closed")
	for exportName := range withoutImportsQueue {
		withoutImports = append(withoutImports, exportName)
	}
	i := 0
	for connection := range connectionsQueue {
		fmt.Println("make graph for ", connection)
		var srcNode *cgraph.Node
		var dstNode *cgraph.Node
		if node, ok := nodeMap[connection.sourceStack]; ok {
			srcNode = node
		} else {
			srcNode, err = graph.CreateNode(connection.sourceStack)
			if err != nil {
				panic(err)
			}
			nodeMap[connection.sourceStack] = srcNode
		}

		if node, ok := nodeMap[connection.destinationStack]; ok {
			dstNode = node
		} else {
			dstNode, err = graph.CreateNode(connection.destinationStack)
			if err != nil {
				panic(err)
			}
			nodeMap[connection.destinationStack] = dstNode
		}

		e2, err := graph.CreateEdge(
			fmt.Sprintf(
				"%d", i,
				),
			dstNode,
			srcNode,
			)
		i += 1
		if err != nil {
			panic(err)
		}
		e2.SetLabel(connection.exportName)
		e2.SetLen(float64(i))
		e2.SetID(fmt.Sprintf("%d", i))
	}
	fmt.Println(exportsList)
	fmt.Println(len(exportsList))
	fmt.Print("Export values without imports: ")
	fmt.Println(withoutImports)
	fmt.Println(len(withoutImports))

	g := graphviz.New()
	defer func() {
		if err := graph.Close(); err != nil {
			log.Fatal(err)
		}
		g.Close()
	}()
	g.RenderFilename(graph, "png", "rw.png")
}

func renderDOTGraph() ([]byte, error) {
	g := graphviz.New()
	graph, err := g.Graph()
	graph = graph.SetESep(30.0)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := graph.Close(); err != nil {
			log.Fatal(err)
		}
		g.Close()
	}()
	var buf bytes.Buffer
	if err := g.Render(graph, "dot", &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
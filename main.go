package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ClusterInfo ...
type ClusterInfo struct {
	Name  string   `json:"cluster_name"`
	Nodes []string `json:"node_list"`
}

// NodeInfo contains info of connectivity on each nodes
type NodeInfo struct {
	Total      int `json:"total"`
	Successful int `json:"successful"`
	Failed     int `json:"failed"`
}

// ClusterResponse ...
type ClusterResponse struct {
	Node NodeInfo `json:"_nodes"`
}

// NetworkTimeout ...
const (
	UpdateInterval       = 15
	NetworkTimeoutResult = 1
)

var (
	port             = flag.String("port", "", "provided port")
	logFile          = flag.String("log-file", "", "Provided log file path")
	targetFolder     = flag.String("folder", "", "provided targert folder")
	timeOutValue     = flag.Int("timeout-value", 2, "timeout value for http client (in seconds)")
	prometheusLabels = []string{"instance", "cluster"}

	esConnectivityFailedGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "elasticsearch_node_connectivity_failed",
			Help: "Elastic Search Node Connectivity Failed",
		},
		prometheusLabels,
	)
	esConnectivityTotalGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "elasticsearch_node_connectivity_total",
			Help: "Elastic Search Node Connectivity Total",
		},
		prometheusLabels,
	)
	esConnectivitySuccessfulGauge = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "elasticsearch_node_connectivity_successful",
			Help: "Elastic Search Node Connectivity Successful",
		},
		prometheusLabels,
	)
)

func main() {
	flag.Parse()

	// Log file
	f, err := os.OpenFile(*logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	log.Println("---------- Start service ---------- ")

	// Update ES nodes status for prometheus to scrape
	go UpdateElasticSearchStatus(*targetFolder)

	router := mux.NewRouter()
	router.Handle("/metrics", promhttp.Handler())
	server := &http.Server{
		Addr:    ":" + *port,
		Handler: router,
	}
	fmt.Println("Running....")
	log.Fatal(server.ListenAndServe())
}

// UpdateElasticSearchStatus update status of nodes connectivity
func UpdateElasticSearchStatus(targetFolder string) {
	ticker := time.NewTicker(time.Second * UpdateInterval)
	for range ticker.C {
		listFile, err := GetFileList(targetFolder)
		if err != nil {
			log.Println("Can't get list files, err = ", err)
			continue
		}

		// Loop thourgh file list
		for _, file := range listFile {
			go func(file os.FileInfo) {
				log.Println("Check file: ", file.Name())
				if file.IsDir() {
					return
				}
				filePath := targetFolder + file.Name()
				clusterInfo, err := LoadConfig(filePath)
				if err != nil {
					log.Printf("Can't load config in file %s, err = %s", file.Name(), err)
					return
				}

				// Loop thourgh nodes
				for _, node := range clusterInfo.Nodes {
					go UpdateNode(node, clusterInfo.Name)
				}
			}(file)
		}
	}
}

// UpdateNode prom metrics of each nodes
func UpdateNode(node, clusterName string) {
	ip, _, err := net.SplitHostPort(node)
	if err != nil {
		return
	}
	nodeInfo := GetNodeInfo(node)

	// Update prom metrics
	prometheusLabels = []string{"ip", "cluster"}
	labels := prometheus.Labels{
		"ip":      ip,
		"cluster": clusterName,
	}
	esConnectivityFailedGauge.With(labels).Set(float64(nodeInfo.Failed))
	esConnectivitySuccessfulGauge.With(labels).Set(float64(nodeInfo.Successful))
	esConnectivityTotalGauge.With(labels).Set(float64(nodeInfo.Total))
	return
}

// GetNodeInfo call ES api to get status of connectiviy
func GetNodeInfo(endpoint string) NodeInfo {
	nodeInfo := NodeInfo{}
	clusterResponse := ClusterResponse{}

	var netClient = &http.Client{
		Timeout: time.Second * time.Duration(*timeOutValue),
	}

	url := fmt.Sprintf("http://" + endpoint + "/_cluster/stats")
	resp, err := netClient.Get(url)
	if err != nil {
		log.Printf("The HTTP request failed with error: %s\n", err)
		nodeInfo.Failed = NetworkTimeoutResult
		return nodeInfo
	}
	defer resp.Body.Close()

	data, _ := ioutil.ReadAll(resp.Body)
	json.Unmarshal([]byte(data), &clusterResponse)
	nodeInfo = clusterResponse.Node
	if err != nil {
		log.Printf("Failed to parse node info result, err: %s\n", err)
	}
	return nodeInfo
}

// LoadConfig parses config from json file
func LoadConfig(file string) (*ClusterInfo, error) {
	clusterInfo := ClusterInfo{}
	configFile, err := os.Open(file)
	defer configFile.Close()

	if err != nil {
		return nil, err
	}

	jsonParser := json.NewDecoder(configFile)
	jsonParser.Decode(&clusterInfo)
	return &clusterInfo, nil
}

// GetFileList returns list of files inside a directory
func GetFileList(dir string) ([]os.FileInfo, error) {
	f, err := os.Open(dir)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	files, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}
	return files, nil
}

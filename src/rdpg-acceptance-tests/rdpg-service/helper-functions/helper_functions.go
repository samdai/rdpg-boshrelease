package helperfunctions

import (
	"fmt"
	"regexp"

	"github.com/starkandwayne/rdpgd/pg"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/starkandwayne/rdpg-acceptance-tests/helpers"
)

// GetRowCount returns the number of rows generated by a given query to the rdpg database
func GetRowCount(address string, sq string) (queryRowCount int, err error) {
	p := pg.NewPG(address, "7432", `rdpg`, `rdpg`, "admin")
	db, err := p.Connect()
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var rowCount []int
	err = db.Select(&rowCount, sq)
	if err != nil {
		return 0, err
	}
	return rowCount[0], nil
}

//GetRowCountUserDB returns the number of rows generated by a give query to a user database
func GetRowCountUserDB(address string, sq string, dbname string) (queryRowCount int, err error) {
	p := pg.NewPG(address, "7432", `rdpg`, dbname, "admin")
	db, err := p.Connect()
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var rowCount []int
	err = db.Select(&rowCount, sq)
	if err != nil {
		return 0, err
	}
	return rowCount[0], nil
}

//ExecQuery - Run a query against the rdpg database
func ExecQuery(address string, sq string) (err error) {
	p := pg.NewPG(address, "7432", `rdpg`, `rdpg`, "admin")
	db, err := p.Connect()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(sq)
	return err
}

//ExecQueryUserDB - Run a query against a user db
func ExecQueryUserDB(address string, sq string, dbname string) (err error) {
	p := pg.NewPG(address, "7432", `rdpg`, dbname, "admin")

	db, err := p.Connect()
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec(sq)
	return err
}

//GetAllNodes queries Consul for all management and service nodes
func GetAllNodes() (allNodes []*consulapi.CatalogService) {

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	nodes, _, _ := consulClient.Catalog().Services(nil)
	re := regexp.MustCompile(`^(rdpg(mc$|sc[0-9]+$))|(sc-([[:alnum:]|-])*m[0-9]+-c[0-9]+$)`)
	for key := range nodes {
		if re.MatchString(key) {
			tempNodes, _, _ := consulClient.Catalog().Service(key, "", nil)
			for index := range tempNodes {
				allNodes = append(allNodes, tempNodes[index])
			}
		}
	}
	return
}

//GetNodesByClusterName return a single catalog service object
func GetNodesByClusterName(clusterName string) (allNodes []*consulapi.CatalogService) {

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	nodes, _, _ := consulClient.Catalog().Service(clusterName, "", nil)

	return nodes

}

//GetServiceNodes returns a list of only service nodes registered in Consul
func GetServiceNodes() (allServiceNodes []*consulapi.CatalogService) {

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	nodes, _, _ := consulClient.Catalog().Services(nil)
	re := regexp.MustCompile(`^(rdpg(sc[0-9]+$))|(sc-([[:alnum:]|-])*m[0-9]+-c[0-9]+$)`)
	for key := range nodes {
		if re.MatchString(key) {
			tempNodes, _, _ := consulClient.Catalog().Service(key, "", nil)
			for index := range tempNodes {
				allServiceNodes = append(allServiceNodes, tempNodes[index])
			}
		}
	}
	return
}

//GetClusterServiceType returns whether the cluster is of type pgbdr or postgresql
func GetClusterServiceType(ClusterID string) (clusterService string) {
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	kv := consulClient.KV()
	key := fmt.Sprintf(`rdpg/%s/cluster/service`, ClusterID)
	kvp, _, err := kv.Get(key, nil)
	if err != nil {
		return
	}
	if kvp != nil {
		clusterService = string(kvp.Value)
	}
	return
}

//GetAllClusterNames returns the name of all clusters (not node names)
func GetAllClusterNames() (allClusterNames []string) {

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	nodes, _, _ := consulClient.Catalog().Services(nil)
	re := regexp.MustCompile(`^(rdpg(mc$|sc[0-9]+$))|(sc-([[:alnum:]|-])*m[0-9]+-c[0-9]+$)`)
	for key := range nodes {
		if re.MatchString(key) {
			allClusterNames = append(allClusterNames, key)
		}
	}
	return
}

//GetList returns 1 column of data for a given sql query
func GetList(address string, sq string) (list []string, err error) {
	p := pg.NewPG(address, "7432", `rdpg`, `rdpg`, "admin")
	db, err := p.Connect()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows := []string{}
	err = db.Select(&rows, sq)
	if err != nil {
		return nil, err
	}
	return rows, nil

}

//GetLeader returns the consul leader (always on the MC nodes)
func GetLeader() string {
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	leader, _ := consulClient.Status().Leader()
	return leader
}

//GetPeers returns the nodes who are not leader
func GetPeers() []string {
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	peers, _ := consulClient.Status().Peers()
	return peers
}

//GetAllNodeNames returns a list of all node names for all clusters
func GetAllNodeNames() (allNodeNames []string) {
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	nodes, _, _ := consulClient.Catalog().Nodes(nil)

	for i := 0; i < len(nodes); i++ {
		allNodeNames = append(allNodeNames, nodes[i].Node)
	}
	return
}

//GetNodeHealthByNodeName runs the consul health checks for a particular node and returns the results
func GetNodeHealthByNodeName(nodeName string) []*consulapi.HealthCheck {
	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = helpers.TestConfig.ConsulIP
	consulClient, _ := consulapi.NewClient(consulConfig)

	healthCheck, _, _ := consulClient.Health().Node(nodeName, nil)

	return healthCheck
}

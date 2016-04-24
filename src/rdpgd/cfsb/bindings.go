package cfsb

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/starkandwayne/rdpgd/globals"
	"github.com/starkandwayne/rdpgd/instances"
	"github.com/starkandwayne/rdpgd/log"
	"github.com/starkandwayne/rdpgd/pg"
	"github.com/starkandwayne/rdpgd/uuid"
)

type Binding struct {
	ID         int          `db:"id"`
	BindingID  string       `db:"binding_id" json:"binding_id"`
	InstanceID string       `db:"instance_id" json:"instance_id"`
	Creds      *Credentials `json:"credentials"`
}

// Create Binding in the data store
func (b *Binding) Create() (err error) {
	log.Trace(fmt.Sprintf(`cfsb.Binding#Create(%s,%s) ... `, b.InstanceID, b.BindingID))

	instance, err := instances.FindByInstanceID(b.InstanceID)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Create(%s) instances.FindByInstanceID(%s) ! %s`, b.BindingID, b.InstanceID, err))
		return
	}

	dns, err := instance.ExternalDNS()
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Create(%s) instance.ExternalDNS(%s) ! %s`, b.BindingID, b.InstanceID, err))
		return
	}
	// For now the crednetials are fixed, future feature is that we will be able to
	// create new users/credentials for each binding later on. Currently only
	// one set of credentials exists for each Instance.
	s := strings.Split(dns, ":")
	uri, err := instance.URI()
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Create(%s) instance.URI(%s) ! %s`, b.BindingID, b.InstanceID, err))
		return
	}
	dsn, err := instance.DSN()
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Create(%s) instance.DSN(%s) ! %s`, b.BindingID, b.InstanceID, err))
		return
	}
	jdbc, err := instance.JDBCURI()
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Create(%s) instance.JDBCURI(%s) ! %s`, b.BindingID, b.InstanceID, err))
		return
	}

	b.Creds = &Credentials{
		InstanceID: b.InstanceID,
		BindingID:  b.BindingID,
		URI:        uri,
		DSN:        dsn,
		JDBCURI:    jdbc,
		Host:       s[0],
		Port:       s[1],
		UserName:   instance.User,
		Password:   instance.Pass,
		Database:   instance.Database,
	}

	p := pg.NewPG(`127.0.0.1`, pbPort, `rdpg`, `rdpg`, pgPass)
	db, err := p.Connect()
	if err != nil {
		log.Error(fmt.Sprintf("cfsb.Binding#Create(%s) ! %s", b.BindingID, err))
		return
	}
	defer db.Close()

	err = b.Find()
	if err != nil {
		if err == sql.ErrNoRows { // Does not yet exist, insert the binding and it's credentials.
			sq := fmt.Sprintf(`INSERT INTO cfsb.bindings (instance_id,binding_id) VALUES (lower('%s'),lower('%s'));`, b.InstanceID, b.BindingID)
			log.Trace(fmt.Sprintf(`cfsb.Binding#Create() > %s`, sq))
			_, err = db.Exec(sq)
			if err != nil {
				log.Error(fmt.Sprintf(`cfsb.Binding#Create(%s) %s ! %s`, b.BindingID, sq, err))
			}
			err := b.Creds.Create()
			if err != nil {
				log.Error(fmt.Sprintf(`cfsb.Binding#Create(%s) b.Creds.Create() ! %s`, b.BindingID, err))
			}
		}
	} else { // Binding already exists, return existing binding and credentials.
		return
	}
	return
}

func (b *Binding) Find() (err error) {
	log.Trace(fmt.Sprintf(`cfsb.Binding#Find(%s) ... `, b.BindingID))

	if b.BindingID == "" {
		return errors.New("Binding ID is empty, can not Binding#Find()")
	}
	p := pg.NewPG(`127.0.0.1`, pbPort, `rdpg`, `rdpg`, pgPass)
	db, err := p.Connect()
	if err != nil {
		log.Error(fmt.Sprintf("cfsb.Binding#Find(%s) ! %s", b.BindingID, err))
		return
	}
	defer db.Close()

	sq := fmt.Sprintf(`SELECT id,instance_id FROM cfsb.bindings WHERE binding_id=lower('%s') LIMIT 1`, b.BindingID)
	log.Trace(fmt.Sprintf(`cfsb.Binding#Find(%s) > %s`, b.BindingID, sq))
	err = db.Get(b, sq)
	if err != nil {
		if err == sql.ErrNoRows {
			log.Error(fmt.Sprintf("cfsb.Binding#Find(%s) ! Could not find binding with given Binding ID", b.BindingID))
		} else {
			log.Error(fmt.Sprintf("cfsb.Binding#Find(%s) ! %s", b.BindingID, err))
		}
	} else {
		// TODO: Load creds: b.Creds := Credentials{} ... b.Creds.Find()
	}
	return
}

func (b *Binding) Remove() (err error) {
	log.Trace(fmt.Sprintf(`cfsb.Binding#Remove(%s) ... `, b.BindingID))
	err = b.Find()
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove(%s) ! %s`, b.BindingID, err))
		return
	}
	p := pg.NewPG(`127.0.0.1`, pbPort, `rdpg`, `rdpg`, pgPass)
	db, err := p.Connect()
	if err != nil {
		log.Error(fmt.Sprintf("cfsb.Binding#Remove(%s) ! %s", b.BindingID, err))
		return
	}
	defer db.Close()

	// TODO: Scheduled background task that does any cleanup necessary for an
	// unbinding (remove credentials?)
	sq := fmt.Sprintf(`UPDATE cfsb.bindings SET ineffective_at=CURRENT_TIMESTAMP WHERE binding_id=lower('%s')`, b.BindingID)
	log.Trace(fmt.Sprintf(`cfsb.Binding#Remove(%s) SQL > %s`, b.BindingID, sq))
	_, err = db.Exec(sq)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove(%s) ! %s`, b.BindingID, err))
	}

	b.Creds = &Credentials{
		InstanceID: b.InstanceID,
		BindingID:  b.BindingID,
	}

	err = b.Creds.Remove()
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove(%s) b.Creds.Remove() ! %s`, b.BindingID, err))
	}

	//The code below this point was added to reset the password on the unbind event

	i, err := instances.FindByInstanceID(b.InstanceID)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove(%s) instances.FindByInstanceID(%s) ! %s`, b.BindingID, b.InstanceID, err))
		return
	}

	if i.ClusterService == `pgbdr` {
		log.Info(fmt.Sprintf(`cfsb.binding#Remove(%s) This is a BDR database, skipping resetting the password, otherwise unbind is complete`, b.BindingID))
		return
	}

	//Get ip of the master for the service cluster
	hostIP, err := getMasterIP(i.ClusterID)
	if err != nil {
		log.Error(fmt.Sprintf("cfsb.Binding#Remove() Could not resolve master ip for database %s on cluster %s, password could not be reset", i.Database, i.ClusterID))
		return err
	}

	//Create new password
	re := regexp.MustCompile("[^A-Za-z0-9_]")
	u2 := uuid.NewUUID().String()
	dbpass := strings.ToLower(string(re.ReplaceAll([]byte(u2), []byte(""))))

	//Connect to the service cluster master node rdpg db
	sc := pg.NewPG(hostIP, globals.PGPort, `rdpg`, `rdpg`, globals.PGPass)
	scdb, err := sc.Connect()
	if err != nil {
		log.Error(fmt.Sprintf("cfsb.Binding#Remove() Attempted to remotely connect to %s ! %s", hostIP, err))
		return
	}
	defer scdb.Close()

	sq = fmt.Sprintf(`ALTER USER %s ENCRYPTED PASSWORD '%s';`, i.User, dbpass)
	_, err = scdb.Exec(sq)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove() Could not update password for db user(%s) ! %s`, i.User, err))
		return
	}

	sq = fmt.Sprintf(`UPDATE cfsb.instances SET dbpass = '%s' WHERE dbname = '%s';`, dbpass, i.Database)
	_, err = scdb.Exec(sq)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove() Could not update password on service cluster %s for db user(%s) ! %s`, i.ClusterID, i.User, err))
		return
	}

	sq = fmt.Sprintf(`INSERT INTO tasks.tasks (cluster_id,node,role,action,data,node_type, cluster_service) VALUES ('%s','%s','service','Reconfigure','pgbouncer','any','%s')`, i.ClusterID, hostIP, i.ClusterService)
	_, err = scdb.Exec(sq)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove() Could not force pgBouncer to reload on service cluster %s for db user(%s) ! %s`, i.ClusterID, i.User, err))
		return
	}

	//Update cfsb.instances on MC node with new password
	sq = fmt.Sprintf(`UPDATE cfsb.instances SET dbpass = '%s' WHERE dbname = '%s';`, dbpass, i.Database)
	_, err = db.Exec(sq)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb.Binding#Remove() Could not update password on the management cluster for db user(%s) ! %s`, i.User, err))
		return
	}

	log.Info(fmt.Sprintf(`cfsb.Binding#Remove(%s) Unbind complete, password reset for user %s`, b.BindingID, i.User))
	return
}

func getMasterIP(clusterName string) (masterIP string, err error) {

	mcConsulIP := `127.0.0.1:8500`

	log.Trace(fmt.Sprintf("cfsb#bindings.getMasterIP() Calling out to Consul at address %s", mcConsulIP))

	consulConfig := consulapi.DefaultConfig()
	consulConfig.Address = mcConsulIP
	consulClient, err := consulapi.NewClient(consulConfig)
	if err != nil {
		log.Error(fmt.Sprintf(`cfsb#bindings.getMasterIP() Consul IP: %s ! %s`, mcConsulIP, err))
		return
	}

	masterNode, _, err := consulClient.Catalog().Service(fmt.Sprintf(`%s-master`, clusterName), "", nil)
	if err != nil {
		log.Error(fmt.Sprintf("cfsb#bindings.getMasterIP() Cluster Name: %s ! %s", clusterName, err))
		return
	}

	if len(masterNode) == 0 {
		masterIP = "0.0.0.0"
		return masterIP, errors.New("Could not find the consul master ip")
	}

	masterIP = masterNode[0].Address
	log.Trace(fmt.Sprintf("cfsb#bindings.getMasterIP() Found master ip for %s = %s", clusterName, masterIP))
	return masterIP, err

}

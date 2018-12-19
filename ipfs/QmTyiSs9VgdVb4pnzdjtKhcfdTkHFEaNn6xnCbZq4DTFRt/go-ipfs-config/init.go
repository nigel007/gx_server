package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"time"
	"net"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"bytes"

	ci "gx/ipfs/QmPvyPwuCgJ7pDmrKDxRtsScJgBaM5h4EpRL2qQJsmXf4n/go-libp2p-crypto"
	peer "gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
)


// add by Nigel start: send restful webservice request
func sendWebServiceRequest(reportRequestItem map[string]interface{}, url string, method string) (map[string]interface{}, error){
	mapResult := make(map[string]interface{})
	bytesData, err := json.Marshal(reportRequestItem)
	if err != nil {
		return mapResult, err
	}
	reader := bytes.NewReader(bytesData)
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		return mapResult, err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return mapResult, err
	}
	if resp.StatusCode != 200 {
		fmt.Println("Error with the request!")
		return mapResult, errors.New("request the server!")
	}
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return mapResult, err
	}
	if err := json.Unmarshal([]byte(string(respBytes)), &mapResult); err != nil {
		return mapResult, err
	}
	return mapResult, nil
}
// add by Nigel end

func Init(out io.Writer, nBitsForKeypair int, serverIp string, serverPort string, username string, password string, webserviceUrl string) (*Config, error) {
	identity, err := identityConfig(out, nBitsForKeypair, serverIp, serverPort, username, password, webserviceUrl)
	if err != nil {
		return nil, err
	}

	bootstrapPeers, err := DefaultBootstrapPeers()
	if err != nil {
		return nil, err
	}

	datastore := DefaultDatastoreConfig()

	conf := &Config{
		API: API{
			HTTPHeaders: map[string][]string{},
		},

		// setup the node's default addresses.
		// NOTE: two swarm listen addrs, one tcp, one utp.
		Addresses: addressesConfig(),

		Datastore: datastore,
		Bootstrap: BootstrapPeerStrings(bootstrapPeers),
		Identity:  identity,
		Discovery: Discovery{
			MDNS: MDNS{
				Enabled:  true,
				Interval: 10,
			},
		},

		Routing: Routing{
			Type: "dht",
		},

		// setup the node mount points.
		Mounts: Mounts{
			IPFS: "/ipfs",
			IPNS: "/ipns",
		},

		Ipns: Ipns{
			ResolveCacheSize: 128,
		},

		Gateway: Gateway{
			RootRedirect: "",
			Writable:     false,
			PathPrefixes: []string{},
			HTTPHeaders: map[string][]string{
				"Access-Control-Allow-Origin":  []string{"*"},
				"Access-Control-Allow-Methods": []string{"GET"},
				"Access-Control-Allow-Headers": []string{"X-Requested-With", "Range"},
			},
		},
		Reprovider: Reprovider{
			Interval: "12h",
			Strategy: "all",
		},
		Swarm: SwarmConfig{
			ConnMgr: ConnMgr{
				LowWater:    DefaultConnMgrLowWater,
				HighWater:   DefaultConnMgrHighWater,
				GracePeriod: DefaultConnMgrGracePeriod.String(),
				Type:        "basic",
			},
		},
	}

	return conf, nil
}

// DefaultConnMgrHighWater is the default value for the connection managers
// 'high water' mark
const DefaultConnMgrHighWater = 900

// DefaultConnMgrLowWater is the default value for the connection managers 'low
// water' mark
const DefaultConnMgrLowWater = 600

// DefaultConnMgrGracePeriod is the default value for the connection managers
// grace period
const DefaultConnMgrGracePeriod = time.Second * 20

func addressesConfig() Addresses {
	return Addresses{
		Swarm: []string{
			"/ip4/0.0.0.0/tcp/4001",
			// "/ip4/0.0.0.0/udp/4002/utp", // disabled for now.
			"/ip6/::/tcp/4001",
		},
		Announce:   []string{},
		NoAnnounce: []string{},
		API:        "/ip4/127.0.0.1/tcp/5001",
		Gateway:    "/ip4/127.0.0.1/tcp/8080",
	}
}

// DefaultDatastoreConfig is an internal function exported to aid in testing.
func DefaultDatastoreConfig() Datastore {
	return Datastore{
		StorageMax:         "10GB",
		StorageGCWatermark: 90, // 90%
		GCPeriod:           "1h",
		BloomFilterSize:    0,
		Spec: map[string]interface{}{
			"type": "mount",
			"mounts": []interface{}{
				map[string]interface{}{
					"mountpoint": "/blocks",
					"type":       "measure",
					"prefix":     "flatfs.datastore",
					"child": map[string]interface{}{
						"type":      "flatfs",
						"path":      "blocks",
						"sync":      true,
						"shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
					},
				},
				map[string]interface{}{
					"mountpoint": "/",
					"type":       "measure",
					"prefix":     "leveldb.datastore",
					"child": map[string]interface{}{
						"type":        "levelds",
						"path":        "datastore",
						"compression": "none",
					},
				},
			},
		},
	}
}

// add by Nigel start: send things to server while initializing
func SendThingsToServerWhileInit(ip_port string, content string) bool {
	conn, err := net.Dial("tcp", ip_port)
	if err != nil {
		fmt.Println("failed to connect to server:", err.Error())
		return false
	}
	conn.Write([]byte(content))
	var response = make([]byte, 1024)
	var count = 0
	for {
		count, err = conn.Read(response)
		if err != nil {
			return false
		} else {
			if string(response[0:count]) == "success" {
				return true
			} else {
				return false
			}
		}
	}
}
// add by Nigel end

// identityConfig initializes a new identity.
func identityConfig(out io.Writer, nbits int, serverIp string, serverPort string, username string, password string, webserviceUrl string) (Identity, error) {
	// TODO guard higher up
	ident := Identity{}
	if nbits < 1024 {
		return ident, errors.New("bitsize less than 1024 is considered unsafe")
	}

	fmt.Fprintf(out, "generating %v-bit RSA keypair...", nbits)
	sk, pk, err := ci.GenerateKeyPair(ci.RSA, nbits)
	if err != nil {
		return ident, err
	}
	fmt.Fprintf(out, "done\n")

	// currently storing key unencrypted. in the future we need to encrypt it.
	// TODO(security)
	skbytes, err := sk.Bytes()
	if err != nil {
		return ident, err
	}
	ident.PrivKey = base64.StdEncoding.EncodeToString(skbytes)

	id, err := peer.IDFromPublicKey(pk)
	if err != nil {
		return ident, err
	}
	ident.PeerID = id.Pretty()

	// add by Nigel start: send the ipfs id to server
	var savedOrNot = SendThingsToServerWhileInit(serverIp+":"+serverPort, "PeerId:"+ident.PeerID)
	if savedOrNot == false{
		return ident, errors.New("not initialized")
	}
	// add by Nigel end

	// add by Nigel start: init with username
	reportRequestItem := make(map[string]interface{})
	reportRequestItem["method"] = "initWithUsername"
	reportRequestItem["username"] = username
	reportRequestItem["password"] = password
	reportRequestItem["nodeId"] = ident.PeerID
	responseResult, err := sendWebServiceRequest(reportRequestItem, webserviceUrl, "POST")
	if err != nil {
		fmt.Println("Error with the network!")
		return ident, errors.New("not initialized")
	}
	responseValue, ok := responseResult["response"]
	if ok {
		if responseValue != "success" {
			fmt.Println("Username and password do not match!")
			return ident, errors.New("not initialized")
		}
	} else {
		fmt.Println("There is something wrong with your request")
		return ident, errors.New("not initialized")
	}
	// add by Nigel end

	fmt.Fprintf(out, "peer identity: %s\n", ident.PeerID)
	return ident, nil
}
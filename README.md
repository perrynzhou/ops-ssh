#### ops-ssh usage
- manager all node meta with encoding
- ssh to every host without password
- privileges control system

##### uage

- vsh_server
```
./vsh_server  -config config.json -p 5566 --dump_minute 3
//-dump_seconds=3 ,timeinvertal for dump all added nodes with encoding
// -config config.json is access user list,jus like
{
  "pub_nodes": [  //public access node list
    "127.0.0.1",
    "92.168.12.1"
  ],
  "user_ref_nodes": [
    {
      "uname": "perrynzhou@192.168.31.162", // usernmae who can access server
      "type": 1  // 1 is super user;0 is normal
    },
    {
      "uname": "root@10.211.55.4", 
      "type": 1,
      "addresses": [ // just uname can access node list,also can access public node list
        "127.0.0.3",
        "127.0.0.4"
      ]
    }
  ]
}
```

- vsh
```
vsh run must with ~/.vsh_config.json,it just like 

{
    "addr":"127.0.0.1", //remote server
    "port":5566   //remote port
}

Usage:
  vsh [command]

Available Commands:
  decode      decode cluster info
  delete      delete nodes of group
  dump        dump cluster info
  go          go host
  help        Help about any command
  list        list {node | group}
  load        load nodes
  template    create  cluster.json 
```
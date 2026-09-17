package nifi

// Responses captured from apache/nifi:latest (NiFi 2.11.0) after seeding a
// flow through its REST API, trimmed to the fields discovery reads. The
// ids are the real ones, so the connections between the fixtures hold.

const (
	fixtureRootID    = "7ed1204d-01a0-1000-fac4-67382d0d2a07"
	fixtureIngestID  = "7ed46465-01a0-1000-665a-2b2250948f6c"
	fixtureDeliverID = "7ed46486-01a0-1000-ffe9-b3f8f6bd1788"
	fixturePoolID    = "7ed466c6-01a0-1000-eaf5-3fd57aa6c1aa"
)

// GET /nifi-api/flow/about
const fixtureAbout = `{"about":{"title":"NiFi","version":"2.11.0","uri":"https://localhost:18443/nifi-api/","contentViewerUrl":"https://localhost:18443/nifi/#/content-viewer","timezone":"UTC","buildTag":"rel/nifi-2.11.0"}}`

// GET /nifi-api/process-groups/root
const fixtureRootGroup = `{
 "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
 "uri": "https://localhost:18443/nifi-api/process-groups/7ed1204d-01a0-1000-fac4-67382d0d2a07",
 "component": {
  "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
  "name": "NiFi Flow",
  "comments": "",
  "runningCount": 1,
  "stoppedCount": 3,
  "invalidCount": 5,
  "disabledCount": 0,
  "inputPortCount": 0,
  "outputPortCount": 0
 }
}`

// GET /nifi-api/flow/process-groups/root
const fixtureRootFlow = `{
 "processGroupFlow": {
  "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
  "uri": "https://localhost:18443/nifi-api/flow/process-groups/7ed1204d-01a0-1000-fac4-67382d0d2a07",
  "breadcrumb": {
   "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
   "breadcrumb": {
    "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
    "name": "NiFi Flow"
   }
  },
  "flow": {
   "processGroups": [
    {
     "id": "7ed46465-01a0-1000-665a-2b2250948f6c",
     "component": {
      "id": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "parentGroupId": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
      "name": "Ingest",
      "comments": "Lands raw orders in S3 and Kafka",
      "runningCount": 1,
      "stoppedCount": 2,
      "invalidCount": 2,
      "disabledCount": 0,
      "inputPortCount": 0,
      "outputPortCount": 1,
      "parameterContext": null
     },
     "runningCount": 1,
     "stoppedCount": 2
    },
    {
     "id": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
     "component": {
      "id": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
      "parentGroupId": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
      "name": "Deliver",
      "comments": "",
      "runningCount": 0,
      "stoppedCount": 2,
      "invalidCount": 3,
      "disabledCount": 0,
      "inputPortCount": 1,
      "outputPortCount": 0,
      "parameterContext": null
     },
     "runningCount": 0,
     "stoppedCount": 2
    }
   ],
   "remoteProcessGroups": [],
   "processors": [],
   "inputPorts": [],
   "outputPorts": [],
   "connections": [
    {
     "id": "7ed4a170-01a0-1000-7824-2f1aa459a038",
     "component": {
      "id": "7ed4a170-01a0-1000-7824-2f1aa459a038",
      "parentGroupId": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
      "name": null,
      "source": {
       "id": "7ed4656a-01a0-1000-e2d5-055b186e6047",
       "type": "OUTPUT_PORT",
       "groupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
       "name": "to-deliver",
       "running": false
      },
      "destination": {
       "id": "7ed46649-01a0-1000-f7d1-923cfffc405d",
       "type": "INPUT_PORT",
       "groupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
       "name": "from-ingest",
       "running": false
      },
      "selectedRelationships": null
     }
    }
   ],
   "labels": [],
   "funnels": []
  },
  "lastRefreshed": "02:25:33 UTC"
 }
}`

// GET /nifi-api/flow/process-groups/{ingest}
const fixtureIngestFlow = `{
 "processGroupFlow": {
  "id": "7ed46465-01a0-1000-665a-2b2250948f6c",
  "uri": "https://localhost:18443/nifi-api/flow/process-groups/7ed46465-01a0-1000-665a-2b2250948f6c",
  "breadcrumb": {
   "id": "7ed46465-01a0-1000-665a-2b2250948f6c",
   "breadcrumb": {
    "id": "7ed46465-01a0-1000-665a-2b2250948f6c",
    "name": "Ingest"
   },
   "parentBreadcrumb": {
    "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
    "breadcrumb": {
     "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
     "name": "NiFi Flow"
    }
   }
  },
  "flow": {
   "processGroups": [],
   "remoteProcessGroups": [],
   "processors": [
    {
     "id": "7ed46544-01a0-1000-1220-0dc1ace8a26a",
     "uri": "https://localhost:18443/nifi-api/processors/7ed46544-01a0-1000-1220-0dc1ace8a26a",
     "component": {
      "id": "7ed46544-01a0-1000-1220-0dc1ace8a26a",
      "parentGroupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "name": "Publish Orders",
      "type": "org.apache.nifi.kafka.processors.PublishKafka",
      "state": "STOPPED",
      "validationStatus": "INVALID",
      "config": {
       "schedulingPeriod": "0 sec",
       "schedulingStrategy": "TIMER_DRIVEN",
       "comments": "",
       "properties": {
        "Topic Name": "orders-events",
        "acks": "all",
        "Transactions Enabled": "true"
       },
       "descriptors": {
        "Topic Name": {
         "name": "Topic Name",
         "displayName": "Topic Name",
         "sensitive": false,
         "required": true
        },
        "acks": {
         "name": "acks",
         "displayName": "Delivery Guarantee",
         "sensitive": false,
         "required": true
        },
        "Transactions Enabled": {
         "name": "Transactions Enabled",
         "displayName": "Transactions Enabled",
         "sensitive": false,
         "required": true
        }
       }
      },
      "relationships": [
       {
        "name": "failure",
        "description": "Any FlowFile that cannot be sent to Kafk",
        "autoTerminate": false
       },
       {
        "name": "success",
        "description": "FlowFiles for which all content was sent",
        "autoTerminate": false
       }
      ]
     },
     "status": {
      "runStatus": "Invalid",
      "validationStatus": null
     }
    },
    {
     "id": "7ed464a7-01a0-1000-a568-021125e71551",
     "uri": "https://localhost:18443/nifi-api/processors/7ed464a7-01a0-1000-a568-021125e71551",
     "component": {
      "id": "7ed464a7-01a0-1000-a568-021125e71551",
      "parentGroupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "name": "Generate Orders",
      "type": "org.apache.nifi.processors.standard.GenerateFlowFile",
      "state": "RUNNING",
      "validationStatus": "VALID",
      "config": {
       "schedulingPeriod": "1 hour",
       "schedulingStrategy": "TIMER_DRIVEN",
       "comments": "",
       "properties": {
        "File Size": "0B",
        "Batch Size": "1"
       },
       "descriptors": {
        "File Size": {
         "name": "File Size",
         "displayName": "File Size",
         "sensitive": false,
         "required": true
        },
        "Batch Size": {
         "name": "Batch Size",
         "displayName": "Batch Size",
         "sensitive": false,
         "required": true
        }
       }
      },
      "relationships": [
       {
        "name": "success",
        "description": "",
        "autoTerminate": false
       }
      ]
     },
     "status": {
      "runStatus": "Running",
      "validationStatus": null
     }
    },
    {
     "id": "7ed464c9-01a0-1000-c17c-8a87e70e76f8",
     "uri": "https://localhost:18443/nifi-api/processors/7ed464c9-01a0-1000-c17c-8a87e70e76f8",
     "component": {
      "id": "7ed464c9-01a0-1000-c17c-8a87e70e76f8",
      "parentGroupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "name": "Stamp Attributes",
      "type": "org.apache.nifi.processors.attributes.UpdateAttribute",
      "state": "STOPPED",
      "validationStatus": "VALID",
      "config": {
       "schedulingPeriod": "0 sec",
       "schedulingStrategy": "TIMER_DRIVEN",
       "comments": "",
       "properties": {
        "source": "orders"
       },
       "descriptors": {
        "source": {
         "name": "source",
         "displayName": "source",
         "sensitive": false,
         "required": false
        }
       }
      },
      "relationships": [
       {
        "name": "success",
        "description": "All successful FlowFiles are routed to t",
        "autoTerminate": false
       }
      ]
     },
     "status": {
      "runStatus": "Stopped",
      "validationStatus": null
     }
    }
   ],
   "inputPorts": [],
   "outputPorts": [
    {
     "id": "7ed4656a-01a0-1000-e2d5-055b186e6047",
     "component": {
      "id": "7ed4656a-01a0-1000-e2d5-055b186e6047",
      "parentGroupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "name": "to-deliver",
      "state": "STOPPED",
      "type": "OUTPUT_PORT",
      "comments": null,
      "portFunction": "STANDARD"
     }
    }
   ],
   "connections": [
    {
     "id": "7ed4662a-01a0-1000-bc12-c43a31173cbd",
     "component": {
      "id": "7ed4662a-01a0-1000-bc12-c43a31173cbd",
      "parentGroupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "name": null,
      "source": {
       "id": "7ed464c9-01a0-1000-c17c-8a87e70e76f8",
       "type": "PROCESSOR",
       "groupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
       "name": "Stamp Attributes",
       "running": false,
       "comments": ""
      },
      "destination": {
       "id": "7ed4656a-01a0-1000-e2d5-055b186e6047",
       "type": "OUTPUT_PORT",
       "groupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
       "name": "to-deliver",
       "running": false
      },
      "selectedRelationships": [
       "success"
      ]
     }
    },
    {
     "id": "7ed46602-01a0-1000-940a-963af7abb325",
     "component": {
      "id": "7ed46602-01a0-1000-940a-963af7abb325",
      "parentGroupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "name": null,
      "source": {
       "id": "7ed464c9-01a0-1000-c17c-8a87e70e76f8",
       "type": "PROCESSOR",
       "groupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
       "name": "Stamp Attributes",
       "running": false,
       "comments": ""
      },
      "destination": {
       "id": "7ed46544-01a0-1000-1220-0dc1ace8a26a",
       "type": "PROCESSOR",
       "groupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
       "name": "Publish Orders",
       "running": false,
       "comments": ""
      },
      "selectedRelationships": [
       "success"
      ]
     }
    },
    {
     "id": "7ed4659a-01a0-1000-f7b1-2137554489dd",
     "component": {
      "id": "7ed4659a-01a0-1000-f7b1-2137554489dd",
      "parentGroupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
      "name": null,
      "source": {
       "id": "7ed464a7-01a0-1000-a568-021125e71551",
       "type": "PROCESSOR",
       "groupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
       "name": "Generate Orders",
       "running": true,
       "comments": ""
      },
      "destination": {
       "id": "7ed464c9-01a0-1000-c17c-8a87e70e76f8",
       "type": "PROCESSOR",
       "groupId": "7ed46465-01a0-1000-665a-2b2250948f6c",
       "name": "Stamp Attributes",
       "running": false,
       "comments": ""
      },
      "selectedRelationships": [
       "success"
      ]
     }
    }
   ],
   "labels": [],
   "funnels": []
  },
  "lastRefreshed": "02:25:34 UTC"
 }
}`

// GET /nifi-api/flow/process-groups/{deliver}
const fixtureDeliverFlow = `{
 "processGroupFlow": {
  "id": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
  "uri": "https://localhost:18443/nifi-api/flow/process-groups/7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
  "breadcrumb": {
   "id": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
   "breadcrumb": {
    "id": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
    "name": "Deliver"
   },
   "parentBreadcrumb": {
    "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
    "breadcrumb": {
     "id": "7ed1204d-01a0-1000-fac4-67382d0d2a07",
     "name": "NiFi Flow"
    }
   }
  },
  "flow": {
   "processGroups": [],
   "remoteProcessGroups": [],
   "processors": [
    {
     "id": "7ed5c4df-01a0-1000-2c7a-2aaee4994681",
     "uri": "https://localhost:18443/nifi-api/processors/7ed5c4df-01a0-1000-2c7a-2aaee4994681",
     "component": {
      "id": "7ed5c4df-01a0-1000-2c7a-2aaee4994681",
      "parentGroupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
      "name": "Notify Webhook",
      "type": "org.apache.nifi.processors.standard.InvokeHTTP",
      "state": "STOPPED",
      "validationStatus": "INVALID",
      "config": {
       "schedulingPeriod": "0 sec",
       "schedulingStrategy": "TIMER_DRIVEN",
       "comments": "Posts each order to the fulfilment webhook",
       "properties": {
        "HTTP Method": "POST",
        "HTTP URL": "https://hooks.example.com/orders",
        "Request Username": "nifi",
        "Request Password": "********"
       },
       "descriptors": {
        "HTTP Method": {
         "name": "HTTP Method",
         "displayName": "HTTP Method",
         "sensitive": false,
         "required": true
        },
        "HTTP URL": {
         "name": "HTTP URL",
         "displayName": "HTTP URL",
         "sensitive": false,
         "required": true
        },
        "Request Username": {
         "name": "Request Username",
         "displayName": "Request Username",
         "sensitive": false,
         "required": false
        },
        "Request Password": {
         "name": "Request Password",
         "displayName": "Request Password",
         "sensitive": true,
         "required": false
        }
       }
      },
      "relationships": [
       {
        "name": "Failure",
        "description": "Request FlowFiles transferred when recei",
        "autoTerminate": false
       },
       {
        "name": "No Retry",
        "description": "Request FlowFiles transferred when recei",
        "autoTerminate": false
       },
       {
        "name": "Original",
        "description": "Request FlowFiles transferred when recei",
        "autoTerminate": false
       },
       {
        "name": "Response",
        "description": "Response FlowFiles transferred when rece",
        "autoTerminate": false
       },
       {
        "name": "Retry",
        "description": "Request FlowFiles transferred when recei",
        "autoTerminate": false
       }
      ]
     },
     "status": {
      "runStatus": "Invalid",
      "validationStatus": null
     }
    },
    {
     "id": "7ed46669-01a0-1000-4461-bb8f5f7931bc",
     "uri": "https://localhost:18443/nifi-api/processors/7ed46669-01a0-1000-4461-bb8f5f7931bc",
     "component": {
      "id": "7ed46669-01a0-1000-4461-bb8f5f7931bc",
      "parentGroupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
      "name": "Log Orders",
      "type": "org.apache.nifi.processors.standard.LogAttribute",
      "state": "STOPPED",
      "validationStatus": "VALID",
      "config": {
       "schedulingPeriod": "0 sec",
       "schedulingStrategy": "TIMER_DRIVEN",
       "comments": "",
       "properties": {
        "Log Level": "info"
       },
       "descriptors": {
        "Log Level": {
         "name": "Log Level",
         "displayName": "Log Level",
         "sensitive": false,
         "required": true
        }
       }
      },
      "relationships": [
       {
        "name": "success",
        "description": "All FlowFiles are routed to this relatio",
        "autoTerminate": false
       }
      ]
     },
     "status": {
      "runStatus": "Stopped",
      "validationStatus": null
     }
    }
   ],
   "inputPorts": [
    {
     "id": "7ed46649-01a0-1000-f7d1-923cfffc405d",
     "component": {
      "id": "7ed46649-01a0-1000-f7d1-923cfffc405d",
      "parentGroupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
      "name": "from-ingest",
      "state": "STOPPED",
      "type": "INPUT_PORT",
      "comments": null,
      "portFunction": "STANDARD"
     }
    }
   ],
   "outputPorts": [],
   "connections": [
    {
     "id": "7ed4a140-01a0-1000-b7c9-d4d9d37018cd",
     "component": {
      "id": "7ed4a140-01a0-1000-b7c9-d4d9d37018cd",
      "parentGroupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
      "name": null,
      "source": {
       "id": "7ed46649-01a0-1000-f7d1-923cfffc405d",
       "type": "INPUT_PORT",
       "groupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
       "name": "from-ingest",
       "running": false
      },
      "destination": {
       "id": "7ed46669-01a0-1000-4461-bb8f5f7931bc",
       "type": "PROCESSOR",
       "groupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
       "name": "Log Orders",
       "running": false,
       "comments": ""
      },
      "selectedRelationships": null
     }
    },
    {
     "id": "7ed5c509-01a0-1000-3607-eb73e116e94a",
     "component": {
      "id": "7ed5c509-01a0-1000-3607-eb73e116e94a",
      "parentGroupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
      "name": null,
      "source": {
       "id": "7ed46669-01a0-1000-4461-bb8f5f7931bc",
       "type": "PROCESSOR",
       "groupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
       "name": "Log Orders",
       "running": false,
       "comments": ""
      },
      "destination": {
       "id": "7ed5c4df-01a0-1000-2c7a-2aaee4994681",
       "type": "PROCESSOR",
       "groupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
       "name": "Notify Webhook",
       "running": false,
       "comments": "Posts each order to the fulfilment webhook"
      },
      "selectedRelationships": [
       "success"
      ]
     }
    }
   ],
   "labels": [],
   "funnels": []
  },
  "lastRefreshed": "02:25:34 UTC"
 }
}`

// GET /nifi-api/controller-services/{pool}
const fixturePool = `{
 "id": "7ed466c6-01a0-1000-eaf5-3fd57aa6c1aa",
 "component": {
  "id": "7ed466c6-01a0-1000-eaf5-3fd57aa6c1aa",
  "parentGroupId": "7ed46486-01a0-1000-ffe9-b3f8f6bd1788",
  "name": "Shop Postgres",
  "type": "org.apache.nifi.dbcp.DBCPConnectionPool",
  "state": "DISABLED",
  "properties": {
   "Database Connection URL": "jdbc:postgresql://db.internal:5432/shop",
   "Database Driver Class Name": "org.postgresql.Driver",
   "Database User": "shop",
   "Password Source": "PASSWORD",
   "Password": "********",
   "Max Wait Time": "500 millis",
   "Max Total Connections": "8",
   "Minimum Idle Connections": "0",
   "Maximum Idle Connections": "8",
   "Maximum Connection Lifetime": "-1",
   "Time Between Eviction Runs": "-1",
   "Minimum Evictable Idle Time": "30 mins",
   "Soft Minimum Evictable Idle Time": "-1"
  },
  "descriptors": {
   "Database Connection URL": {
    "name": "Database Connection URL",
    "sensitive": false
   },
   "Database Driver Class Name": {
    "name": "Database Driver Class Name",
    "sensitive": false
   },
   "Database User": {
    "name": "Database User",
    "sensitive": false
   },
   "Password Source": {
    "name": "Password Source",
    "sensitive": false
   },
   "Password": {
    "name": "Password",
    "sensitive": true
   },
   "Max Wait Time": {
    "name": "Max Wait Time",
    "sensitive": false
   },
   "Max Total Connections": {
    "name": "Max Total Connections",
    "sensitive": false
   },
   "Minimum Idle Connections": {
    "name": "Minimum Idle Connections",
    "sensitive": false
   },
   "Maximum Idle Connections": {
    "name": "Maximum Idle Connections",
    "sensitive": false
   },
   "Maximum Connection Lifetime": {
    "name": "Maximum Connection Lifetime",
    "sensitive": false
   },
   "Time Between Eviction Runs": {
    "name": "Time Between Eviction Runs",
    "sensitive": false
   },
   "Minimum Evictable Idle Time": {
    "name": "Minimum Evictable Idle Time",
    "sensitive": false
   },
   "Soft Minimum Evictable Idle Time": {
    "name": "Soft Minimum Evictable Idle Time",
    "sensitive": false
   }
  }
 }
}`

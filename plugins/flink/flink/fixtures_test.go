package flink

// Responses captured from a Flink 2.3.0 JobManager (docker image flink:latest,
// 2026-09-07) after submitting the bundled streaming examples, trimmed to the
// fields the plugin reads plus a few it ignores.

const clusterConfigJSON = `{
 "refresh-interval": 3000,
 "timezone-name": "Coordinated Universal Time",
 "timezone-offset": 0,
 "flink-version": "2.3.0",
 "flink-revision": "c0f8d1a @ 2026-06-14T16:29:03+02:00",
 "features": {
  "web-submit": true,
  "web-cancel": true,
  "web-rescale": false,
  "web-history": false
 }
}`

const jobsOverviewJSON = `{
 "jobs": [
  {
   "jid": "18c9383da628a78870ee3139d2ad756e",
   "name": "Socket Window WordCount",
   "jobType": "STREAMING",
   "schedulerType": "Default",
   "start-time": 1788815621716,
   "end-time": 1788815622258,
   "duration": 542,
   "state": "FAILED",
   "last-modification": 1788815622258,
   "tasks": {
    "running": 0,
    "canceling": 0,
    "canceled": 1,
    "total": 2,
    "created": 0,
    "scheduled": 0,
    "deploying": 0,
    "reconciling": 0,
    "finished": 0,
    "initializing": 0,
    "failed": 1
   }
  },
  {
   "jid": "3020f12b958bce0bc964c0d9236dc34d",
   "name": "CarTopSpeedWindowingExample",
   "jobType": "STREAMING",
   "schedulerType": "Default",
   "start-time": 1788815571470,
   "end-time": 1788815590790,
   "duration": 19320,
   "state": "CANCELED",
   "last-modification": 1788815590790,
   "tasks": {
    "running": 0,
    "canceling": 0,
    "canceled": 2,
    "total": 2,
    "created": 0,
    "scheduled": 0,
    "deploying": 0,
    "reconciling": 0,
    "finished": 0,
    "initializing": 0,
    "failed": 0
   }
  },
  {
   "jid": "3cea68fa0ce6b9f13db4e9b8aabf45ba",
   "name": "WordCount",
   "jobType": "STREAMING",
   "schedulerType": "Default",
   "start-time": 1788815565832,
   "end-time": 1788815566322,
   "duration": 490,
   "state": "FINISHED",
   "last-modification": 1788815566322,
   "tasks": {
    "running": 0,
    "canceling": 0,
    "canceled": 0,
    "total": 2,
    "created": 0,
    "scheduled": 0,
    "deploying": 0,
    "reconciling": 0,
    "finished": 2,
    "initializing": 0,
    "failed": 0
   }
  },
  {
   "jid": "e4ca10917efb90dc4579d7baad113fac",
   "name": "State machine job",
   "jobType": "STREAMING",
   "schedulerType": "Default",
   "start-time": 1788815560658,
   "end-time": -1,
   "duration": 78729,
   "state": "RUNNING",
   "last-modification": 1788815562342,
   "tasks": {
    "running": 2,
    "canceling": 0,
    "canceled": 0,
    "total": 2,
    "created": 0,
    "scheduled": 0,
    "deploying": 0,
    "reconciling": 0,
    "finished": 0,
    "initializing": 0,
    "failed": 0
   }
  }
 ]
}`

const runningJobJSON = `{
 "jid": "e4ca10917efb90dc4579d7baad113fac",
 "name": "State machine job",
 "isStoppable": false,
 "state": "RUNNING",
 "job-type": "STREAMING",
 "schedulerType": "Default",
 "start-time": 1788815560658,
 "end-time": -1,
 "duration": 30545,
 "maxParallelism": -1,
 "now": 1788815591203,
 "timestamps": {
  "INITIALIZING": 1788815560658,
  "FAILED": 0,
  "SUSPENDED": 0,
  "CANCELLING": 0,
  "CREATED": 1788815561153,
  "RECONCILING": 0,
  "CANCELED": 0,
  "RESTARTING": 0,
  "FINISHED": 0,
  "RUNNING": 1788815561377,
  "FAILING": 0
 },
 "vertices": [
  {
   "id": "bc764cd8ddf7a0cff126f51c16239658",
   "name": "Source: Events Generator Source",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "RUNNING",
   "start-time": 1788815561723,
   "end-time": -1,
   "duration": 29480,
   "metrics": {
    "read-bytes": 0,
    "write-bytes": 618847,
    "read-records": 0,
    "write-records": 28977
   }
  },
  {
   "id": "20ba6b65f97481d5570070de90e4e791",
   "name": "Flat Map -> Sink: Print to Std. Out",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "RUNNING",
   "start-time": 1788815561733,
   "end-time": -1,
   "duration": 29470,
   "metrics": {
    "read-bytes": 634413,
    "write-bytes": 0,
    "read-records": 28897,
    "write-records": 0
   }
  }
 ],
 "plan": {
  "jid": "e4ca10917efb90dc4579d7baad113fac",
  "name": "State machine job",
  "type": "STREAMING",
  "nodes": [
   {
    "id": "20ba6b65f97481d5570070de90e4e791",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "Flat Map<br/>+- Sink: Print to Std. Out<br/>",
    "inputs": [
     {
      "id": "bc764cd8ddf7a0cff126f51c16239658",
      "num": 0,
      "exchange": "pipelined_bounded",
      "ship_strategy": "HASH"
     }
    ]
   },
   {
    "id": "bc764cd8ddf7a0cff126f51c16239658",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "Source: Events Generator Source<br/>"
   }
  ]
 }
}`

const runningConfigJSON = `{
 "jid": "e4ca10917efb90dc4579d7baad113fac",
 "name": "State machine job",
 "execution-config": {
  "restart-strategy": "Cluster level default restart strategy",
  "job-parallelism": 1,
  "object-reuse-mode": false,
  "user-config": {}
 }
}`

const runningExceptionsJSON = `{
 "exceptionHistory": {
  "entries": [],
  "truncated": false
 }
}`

const finishedJobJSON = `{
 "jid": "3cea68fa0ce6b9f13db4e9b8aabf45ba",
 "name": "WordCount",
 "isStoppable": false,
 "state": "FINISHED",
 "job-type": "STREAMING",
 "schedulerType": "Default",
 "start-time": 1788815565832,
 "end-time": 1788815566322,
 "duration": 490,
 "maxParallelism": -1,
 "now": 1788815591434,
 "timestamps": {
  "INITIALIZING": 1788815565832,
  "FAILED": 0,
  "SUSPENDED": 0,
  "CANCELLING": 0,
  "CREATED": 1788815565893,
  "RECONCILING": 0,
  "CANCELED": 0,
  "RESTARTING": 0,
  "FINISHED": 1788815566322,
  "RUNNING": 1788815565902,
  "FAILING": 0
 },
 "vertices": [
  {
   "id": "cbc357ccb763df2852fee8c4fc7d55f2",
   "name": "Source: in-memory-input -> tokenizer",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "FINISHED",
   "start-time": 1788815566116,
   "end-time": 1788815566305,
   "duration": 189,
   "metrics": {
    "read-bytes": 0,
    "write-bytes": 6343,
    "read-records": 0,
    "write-records": 287
   }
  },
  {
   "id": "90bea66de1c231edf33913ecd54406c1",
   "name": "counter -> Sink: print-sink",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "FINISHED",
   "start-time": 1788815566120,
   "end-time": 1788815566314,
   "duration": 194,
   "metrics": {
    "read-bytes": 6356,
    "write-bytes": 0,
    "read-records": 287,
    "write-records": 0
   }
  }
 ],
 "plan": {
  "jid": "3cea68fa0ce6b9f13db4e9b8aabf45ba",
  "name": "WordCount",
  "type": "STREAMING",
  "nodes": [
   {
    "id": "90bea66de1c231edf33913ecd54406c1",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "counter<br/>+- Sink: print-sink<br/>",
    "inputs": [
     {
      "id": "cbc357ccb763df2852fee8c4fc7d55f2",
      "num": 0,
      "exchange": "pipelined_bounded",
      "ship_strategy": "HASH"
     }
    ]
   },
   {
    "id": "cbc357ccb763df2852fee8c4fc7d55f2",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "Source: in-memory-input<br/>+- tokenizer<br/>"
   }
  ]
 }
}`

const finishedConfigJSON = `{
 "jid": "3cea68fa0ce6b9f13db4e9b8aabf45ba",
 "name": "WordCount",
 "execution-config": {
  "restart-strategy": "Cluster level default restart strategy",
  "job-parallelism": 1,
  "object-reuse-mode": false,
  "user-config": {}
 }
}`

const finishedExceptionsJSON = `{
 "exceptionHistory": {
  "entries": [],
  "truncated": false
 }
}`

const canceledJobJSON = `{
 "jid": "3020f12b958bce0bc964c0d9236dc34d",
 "name": "CarTopSpeedWindowingExample",
 "isStoppable": false,
 "state": "CANCELED",
 "job-type": "STREAMING",
 "schedulerType": "Default",
 "start-time": 1788815571470,
 "end-time": 1788815590790,
 "duration": 19320,
 "maxParallelism": -1,
 "now": 1788815591703,
 "timestamps": {
  "INITIALIZING": 1788815571470,
  "FAILED": 0,
  "SUSPENDED": 0,
  "CANCELLING": 1788815590733,
  "CREATED": 1788815571711,
  "RECONCILING": 0,
  "CANCELED": 1788815590790,
  "RESTARTING": 0,
  "FINISHED": 0,
  "RUNNING": 1788815571753,
  "FAILING": 0
 },
 "vertices": [
  {
   "id": "cbc357ccb763df2852fee8c4fc7d55f2",
   "name": "Source: Car data generator source -> Timestamps/Watermarks",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "CANCELED",
   "start-time": 1788815571955,
   "end-time": 1788815590782,
   "duration": 18827,
   "metrics": {
    "read-bytes": 0,
    "write-bytes": 0,
    "read-records": 0,
    "write-records": 187
   }
  },
  {
   "id": "90bea66de1c231edf33913ecd54406c1",
   "name": "GlobalWindows -> Sink: Print to Std. Out",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "CANCELED",
   "start-time": 1788815571960,
   "end-time": 1788815590788,
   "duration": 18828,
   "metrics": {
    "read-bytes": 8069,
    "write-bytes": 0,
    "read-records": 186,
    "write-records": 0
   }
  }
 ],
 "plan": {
  "jid": "3020f12b958bce0bc964c0d9236dc34d",
  "name": "CarTopSpeedWindowingExample",
  "type": "STREAMING",
  "nodes": [
   {
    "id": "90bea66de1c231edf33913ecd54406c1",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "Window(GlobalWindows(trigger=NeverTrigger), DeltaTrigger, TimeEvictor, ComparableAggregator, PassThroughWindowFunction)<br/>+- Sink: Print to Std. Out<br/>",
    "inputs": [
     {
      "id": "cbc357ccb763df2852fee8c4fc7d55f2",
      "num": 0,
      "exchange": "pipelined_bounded",
      "ship_strategy": "HASH"
     }
    ]
   },
   {
    "id": "cbc357ccb763df2852fee8c4fc7d55f2",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "Source: Car data generator source<br/>+- Timestamps/Watermarks<br/>"
   }
  ]
 }
}`

const canceledConfigJSON = `{
 "jid": "3020f12b958bce0bc964c0d9236dc34d",
 "name": "CarTopSpeedWindowingExample",
 "execution-config": {
  "restart-strategy": "Cluster level default restart strategy",
  "job-parallelism": 1,
  "object-reuse-mode": false,
  "user-config": {}
 }
}`

const canceledExceptionsJSON = `{
 "exceptionHistory": {
  "entries": [],
  "truncated": false
 }
}`

const failedJobJSON = `{
 "jid": "18c9383da628a78870ee3139d2ad756e",
 "name": "Socket Window WordCount",
 "isStoppable": false,
 "state": "FAILED",
 "job-type": "STREAMING",
 "schedulerType": "Default",
 "start-time": 1788815621716,
 "end-time": 1788815622258,
 "duration": 542,
 "maxParallelism": -1,
 "now": 1788815639088,
 "timestamps": {
  "INITIALIZING": 1788815621716,
  "FAILED": 1788815622258,
  "SUSPENDED": 0,
  "CANCELLING": 0,
  "CREATED": 1788815621823,
  "RECONCILING": 0,
  "CANCELED": 0,
  "RESTARTING": 0,
  "FINISHED": 0,
  "RUNNING": 1788815621837,
  "FAILING": 1788815622207
 },
 "vertices": [
  {
   "id": "cbc357ccb763df2852fee8c4fc7d55f2",
   "name": "Source: Socket Stream -> Flat Map",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "FAILED",
   "start-time": 1788815621996,
   "end-time": 1788815622185,
   "duration": 189,
   "metrics": {
    "read-bytes": 0,
    "write-bytes": 0,
    "read-records": 0,
    "write-records": 0
   }
  },
  {
   "id": "90bea66de1c231edf33913ecd54406c1",
   "name": "TumblingProcessingTimeWindows -> Sink: Print to Std. Out",
   "maxParallelism": 128,
   "parallelism": 1,
   "status": "CANCELED",
   "start-time": 1788815622000,
   "end-time": 1788815622251,
   "duration": 251,
   "metrics": {
    "read-bytes": 4,
    "write-bytes": 0,
    "read-records": 0,
    "write-records": 0
   }
  }
 ],
 "plan": {
  "jid": "18c9383da628a78870ee3139d2ad756e",
  "name": "Socket Window WordCount",
  "type": "STREAMING",
  "nodes": [
   {
    "id": "90bea66de1c231edf33913ecd54406c1",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "Window(TumblingProcessingTimeWindows(5000), ProcessingTimeTrigger, SocketWindowWordCount$$Lambda$275/0x00000028011b8a30, PassThroughWindowFunction)<br/>+- Sink: Print to Std. Out<br/>",
    "inputs": [
     {
      "id": "cbc357ccb763df2852fee8c4fc7d55f2",
      "num": 0,
      "exchange": "pipelined_bounded",
      "ship_strategy": "HASH"
     }
    ]
   },
   {
    "id": "cbc357ccb763df2852fee8c4fc7d55f2",
    "operator": "",
    "parallelism": 1,
    "operator_strategy": "",
    "description": "Source: Socket Stream<br/>+- Flat Map<br/>"
   }
  ]
 }
}`

const failedConfigJSON = `{
 "jid": "18c9383da628a78870ee3139d2ad756e",
 "name": "Socket Window WordCount",
 "execution-config": {
  "restart-strategy": "Cluster level default restart strategy",
  "job-parallelism": 1,
  "object-reuse-mode": false,
  "user-config": {}
 }
}`

const failedExceptionsJSON = `{
 "exceptionHistory": {
  "entries": [
   {
    "exceptionName": "org.apache.flink.runtime.JobException",
    "stacktrace": "org.apache.flink.runtime.JobException: Recovery is suppressed by NoRestartBackoffTimeStrategy\n\tat org.apache.flink.runtime.executiongraph.failover.ExecutionFailureHandler.handleFailure(ExecutionFailureHandler.java:213)\n\tat org.apache.flink.runtime.executiongraph.failover.ExecutionFailureHandler.handleFailureAndReport(ExecutionFailureHandler.java:163)",
    "timestamp": 1788815622258
   }
  ],
  "truncated": false
 }
}`

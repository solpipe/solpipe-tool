# Proxy


# Testing


## Curl

```bash
grpcurl -plaintext localhost:50151 describe
```

Get:

```
admin.Validator is a service:
service Validator {
  rpc GetDefault ( .admin.Empty ) returns ( .admin.Settings );
  rpc GetLogStream ( .admin.Empty ) returns ( stream .admin.LogLine );
  rpc SetDefault ( .admin.Settings ) returns ( .admin.Settings );
}
grpc.reflection.v1alpha.ServerReflection is a service:
service ServerReflection {
  rpc ServerReflectionInfo ( stream .grpc.reflection.v1alpha.ServerReflectionRequest ) returns ( stream .grpc.reflection.v1alpha.ServerReflectionResponse );
}
job.Work is a service:
service Work {
  rpc GetStatus ( .job.StatusRequest ) returns ( .job.Job );
  rpc SubmitJob ( .job.SubmitRequest ) returns ( stream .job.Job );
}
```


## Admin

RPC calls:

```proto
service Validator{
    // get the default period settings
    rpc GetDefault(Empty) returns (Settings) {}
    // set the default period settings
    rpc SetDefault(Settings) returns (Settings) {}
    // log
    rpc GetLogStream(Empty) returns (stream LogLine) {}
}
```
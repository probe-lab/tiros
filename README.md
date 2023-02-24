# Tiros
[![standard-readme compliant](https://img.shields.io/badge/readme%20style-standard-brightgreen.svg?style=flat-square)](https://github.com/RichardLitt/standard-readme)

Tiros is an IPFS website measurement script. It uses [`clustertest`](https://github.com/guseggert/clustertest) to provision a configurable number of EC2 instances across the world and then uses those to request websites from there. 

## Table of Contents

- [Table of Contents](#table-of-contents)
- [Run](#run)
- [Development](#development)
- [Maintainers](#maintainers)
- [Contributing](#contributing)
- [License](#license)

## Run

You need to provide many configuration parameters to `tiros`. See this help page:

```text
NAME:
   tiros - measures the latency of making requests to the local gateway

USAGE:
   tiros [global options] command [command options] [arguments...]

VERSION:
   0.1.0

COMMANDS:
   help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --versions value [ --versions value ]                                        the kubo versions to test (comma-separated), e.g. 'v0.16.0,v0.17.0'. (default: "v0.17.0") [$TIROS_VERSIONS]
   --nodes-per-version value                                                    the number of nodes per version to run (default: 1) [$TIROS_NODES_PER_VERSION]
   --regions value [ --regions value ]                                          the AWS regions to use, if using an AWS cluster [$TIROS_REGIONS]
   --settle value                                                               the duration to wait after all daemons are online before starting the test (default: 10s) [$TIROS_SETTLE]
   --urls value [ --urls value ]                                                URLs to test against, relative to the gateway URL. Example: '/ipns/ipfs.io' [$TIROS_URLS]
   --times value                                                                number of times to test each URL (default: 5) [$TIROS_TIMES]
   --nodeagent value                                                            path to the nodeagent binary [$TIROS_NODEAGENT_BIN]
   --db-host value                                                              On which host address can this clustertest reach the database [$TIROS_DATABASE_HOST]
   --db-port value                                                              On which port can this clustertest reach the database (default: 0) [$TIROS_DATABASE_PORT]
   --db-name value                                                              The name of the database to use [$TIROS_DATABASE_NAME]
   --db-password value                                                          The password for the database to use [$TIROS_DATABASE_PASSWORD]
   --db-user value                                                              The user with which to access the database to use [$TIROS_DATABASE_USER]
   --db-sslmode value                                                           The sslmode to use when connecting the the database [$TIROS_DATABASE_SSL_MODE]
   --public-subnet-ids value [ --public-subnet-ids value ]                      The public subnet IDs to run the cluster in [$TIROS_PUBLIC_SUBNET_IDS]
   --instance-profile-arns value [ --instance-profile-arns value ]              The instance profiles to run the Kubo nodes with [$TIROS_INSTANCE_PROFILE_ARNS]
   --instance-security-group-ids value [ --instance-security-group-ids value ]  The security groups of the Kubo instances [$TIROS_SECURITY_GROUP_IDS]
   --s3-bucket-arns value [ --s3-bucket-arns value ]                            The S3 buckets where the nodeagent binaries are stored [$TIROS_S3_BUCKET_ARNS]
   --verbose                                                                    Whether to enable verbose logging (default: false) [$TIROS_VERBOSE]
   --help, -h                                                                   show help
   --version, -v                                                                print the version
```

## Development

To create a new migration run:

```shell
migrate create -ext sql -dir migrations -seq create_measurements_table
```

To create the database models

```shell
make models
```

## Maintainers

[@dennis-tra](https://github.com/dennis-tra).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/RichardLitt/standard-readme/issues/new) or submit PRs.

## License

[MIT](LICENSE) © Dennis Trautwein

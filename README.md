# Resque Exporter

Prometheus exporter for Resque metrics.

## Usage

    ./resque-exporter

By default, the resque exporter collects metrics from redis://localhost:6379. You can change it using the `--redis.url` flag.

    ./resque-exporter --redis.url redis://redis.example.com:6379/1

If `REDIS_URL` environment variable is given, it takes precedence over the `--redis.url` flag.

    REDIS_URL=unix:///var/run/redis.sock ./resque-exporter

If your Resque is using a non-default namespace (default is `resque`) to prefix its Redis keys, specify the namespace using the `--redis.namespace` flag.

    ./resque-exporter --redis.namespace app

### Flags

    $ ./resque-exporter --help
    Usage of ./resque-exporter:
      -redis.namespace string
            Namespace used by Resque to prefix all its Redis keys. (default "resque")
      -redis.url string
            URL to the Redis backing the Resque. (default "redis://localhost:6379")
      -version
            Print version information.
      -web.listen-address string
            Address to listen on for web interface and telemetry. (default ":9447")
      -web.telemetry-path string
            Path under which to expose metrics. (default "/metrics")

### Docker

You can deploy the resque exporter using the [zappi/resque-exporter](https://hub.docker.com/r/zappi/resque-exporter/) Docker image.

    docker run -d -p 9447:9447 zappi/resque-exporter --redis.url redis://redis.example.com:6379

## Metrics

| Name | Help | Labels |
| -- | -- | -- |
| resque\_failed\_job\_executions\_total | Total number of failed job executions. | |
| resque\_failed\_scrapes\_total | Total number of failed scrapes. | |
| resque\_job\_executions\_total | Total number of job executions. | |
| resque\_jobs\_in\_failed\_queue | Number of jobs in a failed queue. | queue |
| resque\_jobs\_in\_queue | Number of jobs in a queue. | queue |
| resque\_scrape\_duration\_seconds | Time this scrape of resque metrics took. | |
| resque\_scrapes\_total | Total number of scrapes. | |
| resque\_up | Whether this scrape of resque metrics was successful. | |
| resque\_workers | Number of workers. | |
| resque\_working\_workers | Number of working workers. | |

## Development

### Building

    make

### Building Docker image

    make docker

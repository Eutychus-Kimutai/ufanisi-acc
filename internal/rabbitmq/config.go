package rabbitmq

type RabbitConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Vhost    string `yaml:"vhost"`
	Queues   struct {
		Loan       string `yaml:"loan"`
		Investment string `yaml:"investment"`
		Unresolved string `yaml:"unresolved"`
	} `yaml:"queues"`
	Retry struct {
		MaxRetries   int `yaml:"max_retries"`
		DelaySeconds int `yaml:"delay_seconds"`
	} `yaml:"retry"`
}

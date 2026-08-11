package rabbitmq

type QueueConfig struct {
	Loan                string `yaml:"loan"`
	Investment          string `yaml:"investment"`
	Unresolved          string `yaml:"unresolved"`
	AccrualNotice       string `yaml:"accrual_notice"`
	InvestmentAccrued   string `yaml:"investment_accrued"`
	WithdrawalRequested string `yaml:"withdrawal.requested"`
	WithdrawalProcessed string `yaml:"withdrawal.processed"`
	MaturityNotice      string `yaml:"maturity_notice"`
	Resolved            string `yaml:"resolved"`
}

type RabbitConfig struct {
	Host     string      `yaml:"host"`
	Port     int         `yaml:"port"`
	Username string      `yaml:"username"`
	Password string      `yaml:"password"`
	Vhost    string      `yaml:"vhost"`
	Queues   QueueConfig `yaml:"queues"`
	Retry    struct {
		MaxRetries   int `yaml:"max_retries"`
		DelaySeconds int `yaml:"delay_seconds"`
	} `yaml:"retry"`
}

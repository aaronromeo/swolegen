package config

import "github.com/caarlos0/env/v11"

type Config struct {
	// stateSecret  string `env:"STRAVA_STATE_SECRET,required"`
	// clientID     string `env:"STRAVA_CLIENT_ID,required"`
	// clientSecret string `env:"STRAVA_CLIENT_SECRET,required"`
	// redirectBase string `env:"STRAVA_REDIRECT_BASE_URL,required"`
	// scopes       string `env:"STRAVA_SCOPES,required"`

	LlmRetries       int    `env:"LLM_RETRIES" envDefault:"3"`
	LlmModel         string `env:"LLM_MODEL_ANALYZER"  envDefault:"gpt-4o-mini"`
	LlmMaxFetchBytes int    `env:"LLM_MAX_FETCH_BYTES" envDefault:"65536"`

	OpenaiKey string `env:"OPENAI_API_KEY,required"`
	Debug     bool   `env:"DEBUG" envDefault:"false"`
	Addr      string `env:"ADDR" envDefault:":8080"`
}

func LoadConfig() (*Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

package template

import (
	"echo"
	"os"
	"strings"
)

func EchoTemplateDefaultRegister(
	applicationPrefix string,
	prefixLength uint8,
	logDir string,
) {
	cfg := echo.ConsoleConfig{
		UseColor:   true,
		ShowTime:   true,
		ShowSource: true,
		TimeLayout: "2006-01-02T15:04:05.000Z07:00",
	}
	echo.EchoLogHookRegister(echo.EchoConsoleOutputterCreate(os.Stdout, cfg))
	echo.EchoConfigurationApplicationPrefixSet(applicationPrefix)
	echo.EchoConfigurationMaxPrefixLengthSet(prefixLength)
	echo.EchoSystemInternalLogLevelSet(echo.INFO)
	echo.EchoSystemRegisterDefault(echo.EchoSystemConfiguration{
		MinLogLevel:    echo.DEBUG,
		SystemPrefixes: []string{"Default"},
	})

	cfg.UseColor = false
	fileOutput, err := echo.EchoFileOutputterCreate(
		echo.FileConfig{LogDirectory: logDir, Filename: strings.ToLower(applicationPrefix), MaxFiles: 10, EmbeddedConfig: cfg},
	)
	if err == nil {
		echo.EchoLogHookRegister(fileOutput)
	}
}

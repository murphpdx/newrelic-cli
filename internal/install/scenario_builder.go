package install

import (
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/newrelic/newrelic-cli/internal/install/discovery"
	"github.com/newrelic/newrelic-cli/internal/install/execution"
	"github.com/newrelic/newrelic-cli/internal/install/recipes"
	"github.com/newrelic/newrelic-cli/internal/install/types"
	"github.com/newrelic/newrelic-cli/internal/install/ux"
	"github.com/newrelic/newrelic-cli/internal/install/validation"
	"github.com/newrelic/newrelic-client-go/pkg/nrdb"
)

type TestScenario string

const (
	Basic               TestScenario = "BASIC"
	LogMatches          TestScenario = "LOG_MATCHES"
	Fail                TestScenario = "FAIL"
	StitchedPath        TestScenario = "STITCHED_PATH"
	Canceled            TestScenario = "CANCELED"
	DisplayExplorerLink TestScenario = "DISPLAY_EXPLORER_LINK"
)

var (
	TestScenarios = []TestScenario{
		Basic,
		LogMatches,
		Fail,
		StitchedPath,
		Canceled,
		DisplayExplorerLink,
	}
	emptyResults = []nrdb.NRDBResult{
		map[string]interface{}{
			"count": 0.0,
		},
	}
	nonEmptyResults = []nrdb.NRDBResult{
		map[string]interface{}{
			"count": 1.0,
		},
	}
)

func TestScenarioValues() []string {
	v := make([]string, len(TestScenarios))
	for i, s := range TestScenarios {
		v[i] = string(s)
	}

	return v
}

type ScenarioBuilder struct {
	installerContext InstallerContext
}

func NewScenarioBuilder(ic InstallerContext) *ScenarioBuilder {
	b := ScenarioBuilder{
		installerContext: ic,
	}

	return &b
}

func (b *ScenarioBuilder) BuildScenario(s TestScenario) *RecipeInstaller {
	switch s {
	case Basic:
		return b.Basic()
	case LogMatches:
		return b.LogMatches()
	case Fail:
		return b.Fail()
	case StitchedPath:
		return b.StitchedPath()
	case Canceled:
		return b.CanceledInstall()
	case DisplayExplorerLink:
		return b.DisplayExplorerLink()
	}

	return nil
}

func (b *ScenarioBuilder) Basic() *RecipeInstaller {

	// mock implementations
	rf := setupRecipeFetcherGuidedInstall()
	ers := []execution.StatusSubscriber{
		execution.NewMockStatusReporter(),
		execution.NewTerminalStatusReporter(),
	}
	slg := execution.NewConcreteSuccessLinkGenerator()
	statusRollup := execution.NewInstallStatus(ers, slg)
	c := validation.NewMockNRDBClient()
	c.ReturnResultsAfterNAttempts(emptyResults, nonEmptyResults, 2)
	v := validation.NewPollingRecipeValidator(c)

	pf := discovery.NewRegexProcessFilterer(rf)
	ff := recipes.NewRecipeFileFetcher()
	d := discovery.NewPSUtilDiscoverer(pf)
	gff := discovery.NewGlobFileFilterer()
	re := execution.NewGoTaskRecipeExecutor()
	p := ux.NewPromptUIPrompter()
	s := ux.NewPlainProgress()

	i := RecipeInstaller{
		discoverer:        d,
		fileFilterer:      gff,
		recipeFetcher:     rf,
		recipeExecutor:    re,
		recipeValidator:   v,
		recipeFileFetcher: ff,
		status:            statusRollup,
		prompter:          p,
		progressIndicator: s,
	}

	i.InstallerContext = b.installerContext

	return &i
}

func (b *ScenarioBuilder) Fail() *RecipeInstaller {

	// mock implementations
	rf := setupRecipeFetcherGuidedInstall()
	ers := []execution.StatusSubscriber{
		execution.NewMockStatusReporter(),
		execution.NewTerminalStatusReporter(),
	}
	slg := execution.NewConcreteSuccessLinkGenerator()
	statusRollup := execution.NewInstallStatus(ers, slg)
	c := validation.NewMockNRDBClient()
	c.ReturnResultsAfterNAttempts(emptyResults, nonEmptyResults, 2)
	v := validation.NewPollingRecipeValidator(c)

	pf := discovery.NewRegexProcessFilterer(rf)
	ff := recipes.NewRecipeFileFetcher()
	d := discovery.NewPSUtilDiscoverer(pf)
	gff := discovery.NewGlobFileFilterer()
	re := execution.NewMockFailingRecipeExecutor()
	p := ux.NewPromptUIPrompter()
	pi := ux.NewPlainProgress()

	i := RecipeInstaller{
		discoverer:        d,
		fileFilterer:      gff,
		recipeFetcher:     rf,
		recipeExecutor:    re,
		recipeValidator:   v,
		recipeFileFetcher: ff,
		status:            statusRollup,
		prompter:          p,
		progressIndicator: pi,
	}

	i.InstallerContext = b.installerContext

	return &i
}

func (b *ScenarioBuilder) LogMatches() *RecipeInstaller {

	// mock implementations
	rf := setupRecipeFetcherGuidedInstall()
	ers := []execution.StatusSubscriber{
		execution.NewMockStatusReporter(),
		execution.NewTerminalStatusReporter(),
	}
	slg := execution.NewConcreteSuccessLinkGenerator()
	statusRollup := execution.NewInstallStatus(ers, slg)
	c := validation.NewMockNRDBClient()
	c.ReturnResultsAfterNAttempts(emptyResults, nonEmptyResults, 2)
	v := validation.NewPollingRecipeValidator(c)
	gff := discovery.NewMockFileFilterer()

	pf := discovery.NewRegexProcessFilterer(rf)
	ff := recipes.NewRecipeFileFetcher()
	d := discovery.NewPSUtilDiscoverer(pf)
	re := execution.NewGoTaskRecipeExecutor()
	p := ux.NewPromptUIPrompter()
	pi := ux.NewPlainProgress()

	gff.FilterVal = []types.OpenInstallationLogMatch{
		{
			Name: "asdf",
			File: "asdf",
		},
	}

	i := RecipeInstaller{
		discoverer:        d,
		fileFilterer:      gff,
		recipeFetcher:     rf,
		recipeExecutor:    re,
		recipeValidator:   v,
		recipeFileFetcher: ff,
		status:            statusRollup,
		prompter:          p,
		progressIndicator: pi,
	}

	i.InstallerContext = b.installerContext

	return &i
}

func (b *ScenarioBuilder) StitchedPath() *RecipeInstaller {
	// mock implementations
	rf := setupRecipeFetcherStitchedPath()
	ers := []execution.StatusSubscriber{
		execution.NewMockStatusReporter(),
		execution.NewTerminalStatusReporter(),
	}
	slg := execution.NewConcreteSuccessLinkGenerator()
	statusRollup := execution.NewInstallStatus(ers, slg)
	c := validation.NewMockNRDBClient()
	c.ReturnResultsAfterNAttempts(emptyResults, nonEmptyResults, 2)
	v := validation.NewPollingRecipeValidator(c)

	pf := discovery.NewRegexProcessFilterer(rf)
	ff := recipes.NewRecipeFileFetcher()
	d := discovery.NewPSUtilDiscoverer(pf)
	gff := discovery.NewGlobFileFilterer()
	re := execution.NewGoTaskRecipeExecutor()
	p := ux.NewPromptUIPrompter()
	pi := ux.NewPlainProgress()

	i := RecipeInstaller{
		discoverer:        d,
		fileFilterer:      gff,
		recipeFetcher:     rf,
		recipeExecutor:    re,
		recipeValidator:   v,
		recipeFileFetcher: ff,
		status:            statusRollup,
		prompter:          p,
		progressIndicator: pi,
	}

	i.InstallerContext = b.installerContext

	return &i
}

func (b *ScenarioBuilder) CanceledInstall() *RecipeInstaller {
	// mock implementations
	rf := setupRecipeCanceledInstall()
	ers := []execution.StatusSubscriber{
		execution.NewMockStatusReporter(),
		execution.NewTerminalStatusReporter(),
	}
	slg := execution.NewConcreteSuccessLinkGenerator()
	statusRollup := execution.NewInstallStatus(ers, slg)
	c := validation.NewMockNRDBClient()
	c.ReturnResultsAfterNAttempts(emptyResults, nonEmptyResults, 2)
	v := validation.NewPollingRecipeValidator(c)

	pf := discovery.NewRegexProcessFilterer(rf)
	ff := recipes.NewRecipeFileFetcher()
	d := discovery.NewPSUtilDiscoverer(pf)
	gff := discovery.NewGlobFileFilterer()
	re := execution.NewGoTaskRecipeExecutor()
	p := ux.NewPromptUIPrompter()
	pi := ux.NewPlainProgress()

	i := RecipeInstaller{
		discoverer:        d,
		fileFilterer:      gff,
		recipeFetcher:     rf,
		recipeExecutor:    re,
		recipeValidator:   v,
		recipeFileFetcher: ff,
		status:            statusRollup,
		prompter:          p,
		progressIndicator: pi,
	}

	i.InstallerContext = b.installerContext

	return &i
}

func (b *ScenarioBuilder) DisplayExplorerLink() *RecipeInstaller {
	log.StandardLogger().SetLevel(logrus.DebugLevel)

	// mock implementations
	rf := setupDisplayExplorerLink()
	ers := []execution.StatusSubscriber{
		execution.NewMockStatusReporter(),
		execution.NewTerminalStatusReporter(),
	}
	slg := execution.NewConcreteSuccessLinkGenerator()
	statusRollup := execution.NewInstallStatus(ers, slg)
	c := validation.NewMockNRDBClient()
	c.ReturnResultsAfterNAttempts(emptyResults, nonEmptyResults, 2)
	v := validation.NewPollingRecipeValidator(c)

	pf := discovery.NewRegexProcessFilterer(rf)
	ff := recipes.NewRecipeFileFetcher()
	d := discovery.NewPSUtilDiscoverer(pf)
	gff := discovery.NewGlobFileFilterer()
	re := execution.NewGoTaskRecipeExecutor()
	p := ux.NewPromptUIPrompter()
	pi := ux.NewPlainProgress()

	i := RecipeInstaller{
		discoverer:        d,
		fileFilterer:      gff,
		recipeFetcher:     rf,
		recipeExecutor:    re,
		recipeValidator:   v,
		recipeFileFetcher: ff,
		status:            statusRollup,
		prompter:          p,
		progressIndicator: pi,
	}

	i.InstallerContext = b.installerContext

	return &i
}

func setupRecipeFetcherGuidedInstall() recipes.RecipeFetcher {
	f := recipes.NewMockRecipeFetcher()
	f.FetchRecipeVals = []types.OpenInstallationRecipe{
		{
			Name:        "infrastructure-agent-installer",
			DisplayName: "Infrastructure Agent",
			PreInstall: types.OpenInstallationPreInstallConfiguration{
				Info: `
This is the Infrastructure Agent Installer preinstall message.
It is made up of a multi line string.
				`,
			},
			PostInstall: types.OpenInstallationPostInstallConfiguration{
				Info: `
This is the Infrastructure Agent Installer postinstall message.
It is made up of a multi line string.
				`,
			},
			ValidationNRQL: "test NRQL",
		},
		{
			Name:           "logs-integration",
			DisplayName:    "Logs integration",
			ValidationNRQL: "test NRQL",
			LogMatch: []types.OpenInstallationLogMatch{
				{
					Name: "docker log",
					File: "/var/lib/docker/containers/*/*.log",
				},
			},
		},
	}
	f.FetchRecommendationsVal = []types.OpenInstallationRecipe{
		{
			Name:           "recommended-recipe",
			DisplayName:    "Recommended recipe",
			ValidationNRQL: "test NRQL",
		},
	}

	return f
}

func setupRecipeFetcherStitchedPath() recipes.RecipeFetcher {
	f := recipes.NewMockRecipeFetcher()
	f.FetchRecipeVals = []types.OpenInstallationRecipe{
		{
			Name:           "recommended-recipe",
			DisplayName:    "Recommended recipe",
			ValidationNRQL: "test NRQL",
		},
		{
			Name:           "another-recommended-recipe",
			DisplayName:    "Another Recommended recipe",
			ValidationNRQL: "test NRQL",
		},
	}

	return f
}

func setupRecipeCanceledInstall() recipes.RecipeFetcher {
	f := recipes.NewMockRecipeFetcher()
	f.FetchRecipeVals = []types.OpenInstallationRecipe{
		{
			Name:           "infrastructure-agent-installer",
			DisplayName:    "Infrastructure Agent",
			ValidationNRQL: "test NRQL",
		},
		{
			Name:           "test-canceled-installation",
			DisplayName:    "Test Canceled Installation",
			ValidationNRQL: "test NRQL",
		},
	}

	return f
}

func setupDisplayExplorerLink() recipes.RecipeFetcher {
	f := recipes.NewMockRecipeFetcher()
	f.FetchRecipeVals = []types.OpenInstallationRecipe{
		{
			Name:           "test-display-explorer-link",
			DisplayName:    "Test Display Explorer Link",
			ValidationNRQL: "test NRQL",
			SuccessLinkConfig: types.OpenInstallationSuccessLinkConfig{
				Type:   "explorer",
				Filter: "\"`tags.language` = 'java'\"",
			},
		},
		{
			Name:           "infrastructure-agent-installer",
			DisplayName:    "Infrastructure Agent",
			ValidationNRQL: "test NRQL",
		},
	}

	return f
}

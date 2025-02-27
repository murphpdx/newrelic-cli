package install

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/newrelic/newrelic-cli/internal/install/execution"
	"github.com/newrelic/newrelic-cli/internal/install/types"
	"github.com/newrelic/newrelic-cli/internal/utils"
)

// guidedInstall walks the user through and installation, prompting for input
// when needed.  An error is returned only when the infra or logging recipes
// have an error.  If an OHI recipe fails, we warn the user.  This allows the
// desired user experience.
func (i *RecipeInstaller) guidedInstall(ctx context.Context, m *types.DiscoveryManifest) error {
	var recipesForInstallation []types.OpenInstallationRecipe
	var selectedIntegrations []types.OpenInstallationRecipe
	var recommendedIntegrations []types.OpenInstallationRecipe

	// Fetch the infra agent recipe and mark it as available.
	infraAgentRecipe, err := i.fetchRecipeAndReportAvailable(ctx, m, types.InfraAgentRecipeName)
	if err != nil {
		return err
	}
	recipesForInstallation = append(recipesForInstallation, *infraAgentRecipe)

	// Fetch the logging recipe and mark it as available.
	loggingRecipe, err := i.fetchRecipeAndReportAvailable(ctx, m, types.LoggingRecipeName)
	if err != nil {
		return err
	}

	// Warn the user that --skipInfra is only implemented for targetedInstall
	if i.SkipInfra {
		return fmt.Errorf("--skipInfra is only applicable to targeted installation. Run newrelic install --help for usage")
	}

	// Mark the logging recipe as skipped if necessary.
	if i.SkipLoggingInstall {
		i.status.RecipeSkipped(execution.RecipeStatusEvent{Recipe: *loggingRecipe})
	} else {
		recommendedIntegrations = append(recommendedIntegrations, *loggingRecipe)
	}

	// If necessary, fetch additional integration recommendations from the recipe service.
	if !i.SkipDiscovery {
		var recommended []types.OpenInstallationRecipe
		recommended, err = i.fetchRecommendations(m)
		if err != nil {
			log.Debugf("error fetching additional integrations: %s", err)
			return err
		}

		if len(recommendedIntegrations) == 0 {
			log.Debug("no additional integrations found")
		}

		recommendedIntegrations = append(recommendedIntegrations, recommended...)
	}

	// Filter integrations, based on recipe metadata, command flags and prompts.
	selectedIntegrations, err = i.filterIntegrations(recommendedIntegrations)
	if err != nil {
		return err
	}

	// Mark all recommended integrations as available.
	i.status.RecipesAvailable(selectedIntegrations)

	// Show the user what will be installed.
	recipesForInstallation = append(recipesForInstallation, selectedIntegrations...)
	i.status.RecipesSelected(recipesForInstallation)

	// Remove logging from the integrations list since it will be installed explicitly.
	selectedIntegrations = i.removeRecipes(selectedIntegrations, *loggingRecipe)

	// Install the infra agent.
	log.Debugf("Installing infrastructure agent")
	entityGUID, err := i.executeAndValidateWithProgress(ctx, m, infraAgentRecipe)
	if err != nil {
		log.Error(i.failMessage(types.InfraAgentRecipeName))
		return err
	}
	log.Debugf("Done installing infrastructure agent.")

	// Now that we have a host entity GUID, report recommended integrations
	// with application targets for that host.
	for _, r := range recommendedIntegrations {
		if r.HasApplicationTargetType() {
			i.status.RecipeRecommended(execution.RecipeStatusEvent{
				Recipe:     r,
				EntityGUID: entityGUID,
			})
		}
	}

	// Install logging if necessary.
	if i.ShouldInstallLogging() {
		log.Debugf("Installing logging")
		if err = i.installLogging(ctx, m, loggingRecipe, recipesForInstallation); err != nil {
			log.Error(i.failMessage(types.LoggingRecipeName))
			return err
		}
		log.Debugf("Done installing logging.")
	}

	// Install integrations if necessary, continuing on failure with warnings.
	if i.ShouldInstallIntegrations() {
		log.Debugf("Installing integrations")
		if err = i.installRecipes(ctx, m, selectedIntegrations); err != nil {
			if err == types.ErrInterrupt {
				return err
			}

			return nil
		}
		log.Debugf("Done installing integrations.")
	}

	return nil
}

func (i *RecipeInstaller) installLogging(ctx context.Context, m *types.DiscoveryManifest, r *types.OpenInstallationRecipe, recipes []types.OpenInstallationRecipe) error {
	log.WithFields(log.Fields{
		"recipe_count": len(recipes),
	}).Debug("filtering log matches")
	logMatches, err := i.fileFilterer.Filter(utils.SignalCtx, recipes)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"possible_matches": len(logMatches),
	}).Debug("filtered log matches")

	var acceptedLogMatches []types.OpenInstallationLogMatch
	var ok bool
	for _, match := range logMatches {
		ok, err = i.userAcceptsLogFile(match)
		if err != nil {
			return err
		}

		if ok {
			acceptedLogMatches = append(acceptedLogMatches, match)
		}
	}

	log.WithFields(log.Fields{
		"matches": acceptedLogMatches,
	}).Debug("matches accepted")

	// Build a comma-separated list of discovered log file paths
	discoveredLogFiles := []string{}
	for _, logMatch := range acceptedLogMatches {
		discoveredLogFiles = append(discoveredLogFiles, logMatch.File)
	}

	discoveredLogFilesString := strings.Join(discoveredLogFiles, ",")
	r.SetRecipeVar("NR_DISCOVERED_LOG_FILES", discoveredLogFilesString)

	log.WithFields(log.Fields{
		"NR_DISCOVERED_LOG_FILES": discoveredLogFilesString,
	}).Debug("discovered log files")

	_, err = i.executeAndValidateWithProgress(ctx, m, r)
	return err
}

func (i *RecipeInstaller) fetchRecommendations(m *types.DiscoveryManifest) ([]types.OpenInstallationRecipe, error) {
	log.Debug("fetching recommended recipes")

	recommendations, err := i.recipeFetcher.FetchRecommendations(utils.SignalCtx, m)
	if err != nil {
		return nil, fmt.Errorf("error retrieving recipe recommendations: %s", err)
	}

	recommendations = i.filterRecommendations(recommendations)

	if log.IsLevelEnabled(log.DebugLevel) {
		names := []string{}
		for _, r := range recommendations {
			names = append(names, r.Name)
		}

		log.WithFields(log.Fields{
			"names":        names,
			"recipe_count": len(recommendations),
		}).Debug("recommended integrations")
	}

	return recommendations, nil
}

// Filter out infra and logging recipes from recommendations, since they are
// handled explicitly elsewhere.  This avoids duplicate installation.
func (i *RecipeInstaller) filterRecommendations(recipes []types.OpenInstallationRecipe) []types.OpenInstallationRecipe {
	filteredRecommendations := []types.OpenInstallationRecipe{}
	for _, r := range recipes {
		if r.Name == types.InfraAgentRecipeName || r.Name == types.LoggingRecipeName {
			log.WithFields(log.Fields{
				"name": r.Name,
			}).Debug("skipping redundant recipe")

			continue
		}

		filteredRecommendations = append(filteredRecommendations, r)
	}

	return filteredRecommendations
}

func (i *RecipeInstaller) userAccepts(msg string) (bool, error) {
	if i.AssumeYes {
		return true, nil
	}

	val, err := i.prompter.PromptYesNo(msg)
	if err != nil {
		return false, err
	}

	return val, nil
}

func (i *RecipeInstaller) userAcceptsLogFile(match types.OpenInstallationLogMatch) (bool, error) {
	msg := fmt.Sprintf("Files have been found at the following pattern: %s Do you want to watch them?", match.File)
	return i.userAccepts(msg)
}

func (i *RecipeInstaller) recipeInRecipes(recipe types.OpenInstallationRecipe, recipes []types.OpenInstallationRecipe) bool {
	for _, r := range recipes {
		if recipe.Name == r.Name {
			return true
		}
	}

	return false
}

func (i *RecipeInstaller) removeRecipes(recipes []types.OpenInstallationRecipe, remove ...types.OpenInstallationRecipe) []types.OpenInstallationRecipe {
	filtered := []types.OpenInstallationRecipe{}
	for _, recipe := range recipes {
		for _, r := range remove {
			if recipe.Name != r.Name {
				filtered = append(filtered, recipe)
			}
		}
	}

	return filtered
}

// filterIntegration has several purposes:
//   - create a filtered list of install candidates based on command flags and user prompt input
//   - mark recipes as SKIPPED based on the SkipIntegrations command flag
//   - mark recipes as SKIPPED if designated by user prompt input
//   - ensure the logging recipe is skipped if designated by user prompt input
//   - filter out recipes with APPLICATION target types
func (i *RecipeInstaller) filterIntegrations(recommendedIntegrations []types.OpenInstallationRecipe) ([]types.OpenInstallationRecipe, error) {
	installCandidates := []types.OpenInstallationRecipe{}
	for _, r := range recommendedIntegrations {
		if r.HasApplicationTargetType() && !r.IsApm() {
			// do nothing
		} else if i.SkipIntegrations {
			i.status.RecipeSkipped(execution.RecipeStatusEvent{Recipe: r})
		} else if i.SkipApm && r.IsApm() {
			i.status.RecipeSkipped(execution.RecipeStatusEvent{Recipe: r})
		} else {
			installCandidates = append(installCandidates, r)
		}
	}

	installCandidateNames := []string{}
	for _, r := range installCandidates {
		installCandidateNames = append(installCandidateNames, r.DisplayName)
	}

	var selectedIntegrationNames []string
	if i.AssumeYes {
		// When -y is supplied, select all the recipes that were in the report for install.
		selectedIntegrationNames = installCandidateNames
	} else if len(installCandidateNames) > 0 {
		fmt.Printf("The guided installation will begin by installing the latest version of the New Relic Infrastructure agent, which is required for additional instrumentation.\n\n")

		var promptErr error
		selectedIntegrationNames, promptErr = i.prompter.MultiSelect("Please choose from the additional recommended instrumentation to be installed:", installCandidateNames)
		if promptErr != nil {
			return nil, promptErr
		}

		fmt.Println()
	}

	var integrationsForInstall []types.OpenInstallationRecipe
	for _, selectedIntegrationName := range selectedIntegrationNames {
		for _, r := range recommendedIntegrations {
			if r.DisplayName == selectedIntegrationName {
				integrationsForInstall = append(integrationsForInstall, r)
			}
		}
	}

	log.Debug("skipping recipes that were not selected")
	for _, r := range installCandidates {
		if !i.recipeInRecipes(r, integrationsForInstall) {
			i.status.RecipeSkipped(execution.RecipeStatusEvent{Recipe: r})

			if r.Name == types.LoggingRecipeName {
				i.SkipLoggingInstall = true
			}
		}
	}

	return integrationsForInstall, nil
}

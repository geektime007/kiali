package checkers

import (
	"github.com/kiali/kiali/business/checkers/common"
	"github.com/kiali/kiali/business/checkers/destinationrules"
	"github.com/kiali/kiali/kubernetes"
	"github.com/kiali/kiali/models"
)

const DestinationRuleCheckerType = "destinationrule"

type DestinationRulesChecker struct {
	DestinationRules         []kubernetes.IstioObject
	ExportedDestinationRules []kubernetes.IstioObject
	MTLSDetails              kubernetes.MTLSDetails
	ServiceEntries           []kubernetes.IstioObject
	Namespaces               []models.Namespace
}

func (in DestinationRulesChecker) Check() models.IstioValidations {
	validations := models.IstioValidations{}

	validations = validations.MergeValidations(in.runIndividualChecks())
	validations = validations.MergeValidations(in.runGroupChecks())

	return validations
}

func (in DestinationRulesChecker) runGroupChecks() models.IstioValidations {
	validations := models.IstioValidations{}

	seHosts := kubernetes.ServiceEntryHostnames(in.ServiceEntries)

	enabledDRCheckers := []GroupChecker{
		destinationrules.MultiMatchChecker{Namespaces: in.Namespaces, DestinationRules: in.DestinationRules, ServiceEntries: seHosts, ExportedDestinationRules: in.ExportedDestinationRules},
	}

	// Appending validations that only applies to non-autoMTLS meshes
	if !in.MTLSDetails.EnabledAutoMtls {
		enabledDRCheckers = append(enabledDRCheckers, destinationrules.TrafficPolicyChecker{DestinationRules: in.DestinationRules, MTLSDetails: in.MTLSDetails})
	}

	for _, checker := range enabledDRCheckers {
		validations = validations.MergeValidations(checker.Check())
	}

	return validations
}

func (in DestinationRulesChecker) runIndividualChecks() models.IstioValidations {
	validations := models.IstioValidations{}

	for _, destinationRule := range in.DestinationRules {
		validations.MergeValidations(in.runChecks(destinationRule))
	}

	return validations
}

func (in DestinationRulesChecker) runChecks(destinationRule kubernetes.IstioObject) models.IstioValidations {
	destinationRuleName := destinationRule.GetObjectMeta().Name
	key, rrValidation := EmptyValidValidation(destinationRuleName, destinationRule.GetObjectMeta().Namespace, DestinationRuleCheckerType)

	enabledCheckers := []Checker{
		destinationrules.DisabledNamespaceWideMTLSChecker{DestinationRule: destinationRule, MTLSDetails: in.MTLSDetails},
		destinationrules.DisabledMeshWideMTLSChecker{DestinationRule: destinationRule, MeshPeerAuthns: in.MTLSDetails.MeshPeerAuthentications},
		common.ExportToNamespaceChecker{IstioObject: destinationRule, Namespaces: in.Namespaces},
	}

	// Appending validations that only applies to non-autoMTLS meshes
	if !in.MTLSDetails.EnabledAutoMtls {
		enabledCheckers = append(enabledCheckers, destinationrules.NamespaceWideMTLSChecker{DestinationRule: destinationRule, MTLSDetails: in.MTLSDetails})
		enabledCheckers = append(enabledCheckers, destinationrules.MeshWideMTLSChecker{DestinationRule: destinationRule, MTLSDetails: in.MTLSDetails})
	}

	for _, checker := range enabledCheckers {
		checks, validChecker := checker.Check()
		rrValidation.Checks = append(rrValidation.Checks, checks...)
		rrValidation.Valid = rrValidation.Valid && validChecker
	}

	return models.IstioValidations{key: rrValidation}
}

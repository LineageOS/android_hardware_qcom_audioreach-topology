//
// SPDX-FileCopyrightText: The LineageOS Project
// SPDX-License-Identifier: Apache-2.0
//

package audioreach_topology

import (
	"github.com/google/blueprint"
	"github.com/google/blueprint/proptools"

	"android/soong/android"
)

var (
	pctx = android.NewPackageContext("android/soong/audioreach_topology")
)

func init() {
	RegisterBuildComponents(android.InitRegistrationContext)

	pctx.Import("android/soong/android")
}

func RegisterBuildComponents(ctx android.RegistrationContext) {
	ctx.RegisterModuleType("audioreach_topology", audioreachTopologyFactory)
	ctx.RegisterModuleType("audioreach_topology_defaults", audioreachTopologyDefaultsFactory)

	ctx.FinalDepsMutators(func(ctx android.RegisterMutatorsContext) {
		ctx.BottomUp("audioreach_topology_deps", audioreachTopologyDepsMutator)
	})
}

type audioreachTopologyProperties struct {
	// The .m4 topology entry point file to compile
	Topology *string `android:"path"`

	// Include .m4 files (e.g. audioreach/**/*.m4, util/**/*.m4).
	Includes []string `android:"path"`

	// Output file name. Defaults to {name}.mbn
	Stem *string

	// Install to a subdirectory of the install path
	Relative_install_path *string
}

type audioreachTopology struct {
	android.ModuleBase
	android.DefaultableModuleBase
	properties audioreachTopologyProperties

	outputFile  android.WritablePath
	installPath android.InstallPath
}

type audioreachTopologyDefaults struct {
	android.ModuleBase
	android.DefaultsModuleBase
	properties audioreachTopologyProperties
}

func audioreachTopologyFactory() android.Module {
	module := &audioreachTopology{}
	module.AddProperties(&module.properties)
	android.InitAndroidArchModule(module, android.DeviceSupported, android.MultilibCommon)
	android.InitDefaultableModule(module)
	return module
}

func audioreachTopologyDefaultsFactory() android.Module {
	module := &audioreachTopologyDefaults{}
	module.AddProperties(&module.properties)
	android.InitDefaultsModule(module)
	return module
}

func (a *audioreachTopology) InstallInVendor() bool {
	return true
}

type hostToolDepTag struct {
	blueprint.BaseDependencyTag
}

var alsatplgDepTag = hostToolDepTag{}

func audioreachTopologyDepsMutator(ctx android.BottomUpMutatorContext) {
	if _, ok := ctx.Module().(*audioreachTopology); ok {
		ctx.AddFarVariationDependencies(ctx.Config().BuildOSTarget.Variations(), alsatplgDepTag, "alsatplg")
	}
}

func (a *audioreachTopology) GenerateAndroidBuildActions(ctx android.ModuleContext) {
	if a.properties.Topology == nil {
		ctx.PropertyErrorf("topology", "must specify a topology .m4 file")
		return
	}
	topologyPath := android.PathForModuleSrc(ctx, proptools.String(a.properties.Topology))
	if topologyPath.Ext() != ".m4" {
		ctx.PropertyErrorf("topology", "expected a .m4 file, got %q", topologyPath.Base())
		return
	}

	if len(a.properties.Includes) == 0 {
		ctx.PropertyErrorf("includes", "must specify at least one include .m4 file")
		return
	}

	includePaths := android.PathsForModuleSrc(ctx, a.properties.Includes)
	for _, inc := range includePaths {
		if inc.Ext() != ".m4" {
			ctx.PropertyErrorf("includes", "expected .m4 files, got %q", inc.Base())
			return
		}
	}

	var alsatplgPath android.Path

	ctx.VisitDirectDepsProxyAllowDisabled(func(proxy android.ModuleProxy) {
		if ctx.OtherModuleDependencyTag(proxy) == alsatplgDepTag {
			module := android.PrebuiltGetPreferred(ctx, proxy)

			if h, ok := android.OtherModuleProvider(ctx, module, android.HostToolProviderInfoProvider); ok {
				if h.HostToolPath.Valid() {
					alsatplgPath = h.HostToolPath.Path()
				}
			}
		}
	})

	if alsatplgPath == nil {
		ctx.ModuleErrorf("Failed to resolve host tool 'alsatplg'. Did the mutator run?")
		return
	}

	stem := proptools.StringDefault(a.properties.Stem, ctx.ModuleName()+".tplg")
	a.outputFile = android.PathForModuleOut(ctx, stem)

	confFile := android.PathForModuleOut(ctx, ctx.ModuleName()+".conf")

	rule := android.NewRuleBuilder(pctx, ctx)

	rule.Command().
		PrebuiltBuildTool(ctx, "m4").
		Text("--fatal-warnings -s").
		Flag("-I "+ctx.ModuleDir()).
		Input(topologyPath).
		Implicits(includePaths).
		FlagWithOutput("> ", confFile)

	rule.Command().
		Tool(alsatplgPath).
		FlagWithInput("-c ", confFile).
		FlagWithOutput("-o ", a.outputFile)

	rule.Build("audioreach_topology", "Compiling topology")

	a.installPath = android.PathForModuleInstall(ctx, "firmware")
	if subdir := proptools.String(a.properties.Relative_install_path); subdir != "" {
		a.installPath = a.installPath.Join(ctx, subdir)
	}

	ctx.InstallFile(a.installPath, a.outputFile.Base(), a.outputFile)
	ctx.SetOutputFiles(android.Paths{a.outputFile}, "")
}

func (a *audioreachTopology) AndroidMkEntries() []android.AndroidMkEntries {
	if a.outputFile == nil {
		return nil
	}

	return []android.AndroidMkEntries{{
		Class:      "ETC",
		OutputFile: android.OptionalPathForPath(a.outputFile),
		ExtraEntries: []android.AndroidMkExtraEntriesFunc{
			func(ctx android.AndroidMkExtraEntriesContext, entries *android.AndroidMkEntries) {
				entries.SetPath("LOCAL_MODULE_PATH", a.installPath)
				entries.SetString("LOCAL_INSTALLED_MODULE_STEM", a.outputFile.Base())
			},
		},
	},
	}
}

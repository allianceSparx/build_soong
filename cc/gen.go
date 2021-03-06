// Copyright 2015 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cc

import (
	"path/filepath"

	"github.com/google/blueprint"

	"android/soong/android"
)

func init() {
	pctx.VariableFunc("GetBisonPath", func(ctx android.PackageVarContext) string {
		if override := ctx.Config().Getenv("USE_HOST_BISON"); override == "yes" {
			return "${HostOutExecutable}/bison"
		}
		return "prebuilts/build-tools/${config.HostPrebuiltTag}/bin/bison"
	})

	pctx.SourcePathVariable("lexCmd", "prebuilts/build-tools/${config.HostPrebuiltTag}/bin/flex")
	pctx.SourcePathVariable("yaccCmd", "${GetBisonPath}")

	pctx.SourcePathVariable("yaccDataDir", "prebuilts/build-tools/common/bison")

	pctx.HostBinToolVariable("aidlCmd", "aidl-cpp")
	pctx.HostBinToolVariable("syspropCmd", "sysprop_cpp")
}

var (
	yacc = pctx.AndroidStaticRule("yacc",
		blueprint.RuleParams{
			Command:     "BISON_PKGDATADIR=$yaccDataDir $yaccCmd -d $yaccFlags --defines=$hFile -o $out $in",
			CommandDeps: []string{"$yaccCmd"},
		},
		"yaccFlags", "hFile")

	lex = pctx.AndroidStaticRule("lex",
		blueprint.RuleParams{
			Command:     "$lexCmd -o$out $in",
			CommandDeps: []string{"$lexCmd"},
		})

	aidl = pctx.AndroidStaticRule("aidl",
		blueprint.RuleParams{
			Command:     "$aidlCmd -d${out}.d --ninja $aidlFlags $in $outDir $out",
			CommandDeps: []string{"$aidlCmd"},
			Depfile:     "${out}.d",
			Deps:        blueprint.DepsGCC,
		},
		"aidlFlags", "outDir")

	sysprop = pctx.AndroidStaticRule("sysprop",
		blueprint.RuleParams{
			Command: "$syspropCmd --header-dir=$headerOutDir --system-header-dir=$systemOutDir " +
				"--source-dir=$srcOutDir --include-name=$includeName $in",
			CommandDeps: []string{"$syspropCmd"},
		},
		"headerOutDir", "systemOutDir", "srcOutDir", "includeName")

	windmc = pctx.AndroidStaticRule("windmc",
		blueprint.RuleParams{
			Command:     "$windmcCmd -r$$(dirname $out) -h$$(dirname $out) $in",
			CommandDeps: []string{"$windmcCmd"},
		},
		"windmcCmd")
)

func genYacc(ctx android.ModuleContext, yaccFile android.Path, outFile android.ModuleGenPath, yaccFlags string) (headerFile android.ModuleGenPath) {
	headerFile = android.GenPathWithExt(ctx, "yacc", yaccFile, "h")

	ctx.Build(pctx, android.BuildParams{
		Rule:           yacc,
		Description:    "yacc " + yaccFile.Rel(),
		Output:         outFile,
		ImplicitOutput: headerFile,
		Input:          yaccFile,
		Args: map[string]string{
			"yaccFlags": yaccFlags,
			"hFile":     headerFile.String(),
		},
	})

	return headerFile
}

func genAidl(ctx android.ModuleContext, aidlFile android.Path, outFile android.ModuleGenPath, aidlFlags string) android.Paths {
	ctx.Build(pctx, android.BuildParams{
		Rule:        aidl,
		Description: "aidl " + aidlFile.Rel(),
		Output:      outFile,
		Input:       aidlFile,
		Args: map[string]string{
			"aidlFlags": aidlFlags,
			"outDir":    android.PathForModuleGen(ctx, "aidl").String(),
		},
	})

	// TODO: This should return the generated headers, not the source file.
	return android.Paths{outFile}
}

func genLex(ctx android.ModuleContext, lexFile android.Path, outFile android.ModuleGenPath) {
	ctx.Build(pctx, android.BuildParams{
		Rule:        lex,
		Description: "lex " + lexFile.Rel(),
		Output:      outFile,
		Input:       lexFile,
	})
}

func genSysprop(ctx android.ModuleContext, syspropFile android.Path) (android.Path, android.Path) {
	headerFile := android.PathForModuleGen(ctx, "sysprop", "include", syspropFile.Rel()+".h")
	systemHeaderFile := android.PathForModuleGen(ctx, "sysprop/system", "include", syspropFile.Rel()+".h")
	cppFile := android.PathForModuleGen(ctx, "sysprop", syspropFile.Rel()+".cpp")

	ctx.Build(pctx, android.BuildParams{
		Rule:           sysprop,
		Description:    "sysprop " + syspropFile.Rel(),
		Output:         cppFile,
		ImplicitOutput: headerFile,
		Input:          syspropFile,
		Args: map[string]string{
			"headerOutDir": filepath.Dir(headerFile.String()),
			"systemOutDir": filepath.Dir(systemHeaderFile.String()),
			"srcOutDir":    filepath.Dir(cppFile.String()),
			"includeName":  syspropFile.Rel() + ".h",
		},
	})

	return cppFile, headerFile
}

func genWinMsg(ctx android.ModuleContext, srcFile android.Path, flags builderFlags) (android.Path, android.Path) {
	headerFile := android.GenPathWithExt(ctx, "windmc", srcFile, "h")
	rcFile := android.GenPathWithExt(ctx, "windmc", srcFile, "rc")

	windmcCmd := gccCmd(flags.toolchain, "windmc")

	ctx.Build(pctx, android.BuildParams{
		Rule:           windmc,
		Description:    "windmc " + srcFile.Rel(),
		Output:         rcFile,
		ImplicitOutput: headerFile,
		Input:          srcFile,
		Args: map[string]string{
			"windmcCmd": windmcCmd,
		},
	})

	return rcFile, headerFile
}

func genSources(ctx android.ModuleContext, srcFiles android.Paths,
	buildFlags builderFlags) (android.Paths, android.Paths) {

	var deps android.Paths

	var rsFiles android.Paths

	for i, srcFile := range srcFiles {
		switch srcFile.Ext() {
		case ".y":
			cFile := android.GenPathWithExt(ctx, "yacc", srcFile, "c")
			srcFiles[i] = cFile
			deps = append(deps, genYacc(ctx, srcFile, cFile, buildFlags.yaccFlags))
		case ".yy":
			cppFile := android.GenPathWithExt(ctx, "yacc", srcFile, "cpp")
			srcFiles[i] = cppFile
			deps = append(deps, genYacc(ctx, srcFile, cppFile, buildFlags.yaccFlags))
		case ".l":
			cFile := android.GenPathWithExt(ctx, "lex", srcFile, "c")
			srcFiles[i] = cFile
			genLex(ctx, srcFile, cFile)
		case ".ll":
			cppFile := android.GenPathWithExt(ctx, "lex", srcFile, "cpp")
			srcFiles[i] = cppFile
			genLex(ctx, srcFile, cppFile)
		case ".proto":
			ccFile, headerFile := genProto(ctx, srcFile, buildFlags)
			srcFiles[i] = ccFile
			deps = append(deps, headerFile)
		case ".aidl":
			cppFile := android.GenPathWithExt(ctx, "aidl", srcFile, "cpp")
			srcFiles[i] = cppFile
			deps = append(deps, genAidl(ctx, srcFile, cppFile, buildFlags.aidlFlags)...)
		case ".rs", ".fs":
			cppFile := rsGeneratedCppFile(ctx, srcFile)
			rsFiles = append(rsFiles, srcFiles[i])
			srcFiles[i] = cppFile
		case ".mc":
			rcFile, headerFile := genWinMsg(ctx, srcFile, buildFlags)
			srcFiles[i] = rcFile
			deps = append(deps, headerFile)
		case ".sysprop":
			cppFile, headerFile := genSysprop(ctx, srcFile)
			srcFiles[i] = cppFile
			deps = append(deps, headerFile)
		}
	}

	if len(rsFiles) > 0 {
		deps = append(deps, rsGenerateCpp(ctx, rsFiles, buildFlags.rsFlags)...)
	}

	return srcFiles, deps
}

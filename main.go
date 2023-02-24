package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
)

const _defaultMemLimit = 0xFFFF

var output = flag.String("o", "main", "output")

type loop struct {
	body *ir.Block
	end  *ir.Block
}

func usage() {
	fmt.Fprintf(os.Stderr, "usage: bfc [-o output] <file>\n")
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("bfc: ")

	flag.Usage = usage
	flag.Parse()

	if flag.NArg() != 1 {
		usage()
		os.Exit(2)
	}

	prog, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}

	mod := ir.NewModule()

	getchar := mod.NewFunc("getchar", types.I8)
	putchar := mod.NewFunc("putchar", types.I32, ir.NewParam("ch", types.I8))
	memset := mod.NewFunc("memset", types.Void, ir.NewParam("ptr", types.I8Ptr), ir.NewParam("val", types.I8), ir.NewParam("len", types.I64))

	mainFn := mod.NewFunc("main", types.I32)
	block := mainFn.NewBlock("")

	arrayType := types.NewArray(_defaultMemLimit, types.I8)
	cellMem := block.NewAlloca(arrayType)

	block.NewCall(memset,
		block.NewGetElementPtr(arrayType, cellMem, constant.NewInt(types.I64, 0), constant.NewInt(types.I64, 0)),
		constant.NewInt(types.I8, 0),
		constant.NewInt(types.I64, _defaultMemLimit))

	pc := block.NewAlloca(types.I64)
	block.NewStore(constant.NewInt(types.I64, 0), pc)
	var stack []loop

	for i := 0; i < len(prog); i++ {
		switch prog[i] {
		case '+':
			ptr := block.NewGetElementPtr(arrayType, cellMem, constant.NewInt(types.I64, 0), block.NewLoad(types.I64, pc))
			added := block.NewAdd(block.NewLoad(types.I8, ptr), constant.NewInt(types.I8, 1))
			block.NewStore(added, ptr)
		case '-':
			ptr := block.NewGetElementPtr(arrayType, cellMem, constant.NewInt(types.I64, 0), block.NewLoad(types.I64, pc))
			added := block.NewAdd(block.NewLoad(types.I8, ptr), constant.NewInt(types.I8, -1))
			block.NewStore(added, ptr)
		case '<':
			t1 := block.NewAdd(block.NewLoad(types.I64, pc), constant.NewInt(types.I8, -1))
			block.NewStore(t1, pc)
		case '>':
			t1 := block.NewAdd(block.NewLoad(types.I64, pc), constant.NewInt(types.I8, 1))
			block.NewStore(t1, pc)
		case '.':
			ptr := block.NewGetElementPtr(arrayType, cellMem, constant.NewInt(types.I64, 0), block.NewLoad(types.I64, pc))
			block.NewCall(putchar, block.NewLoad(types.I8, ptr))
		case ',':
			char := block.NewCall(getchar)
			ptr := block.NewGetElementPtr(arrayType, cellMem, constant.NewInt(types.I64, 0), block.NewLoad(types.I64, pc))
			block.NewStore(char, ptr)
		case '[':
			ptr := block.NewGetElementPtr(arrayType, cellMem, constant.NewInt(types.I64, 0), block.NewLoad(types.I64, pc))
			ld := block.NewLoad(types.I8, ptr)

			cmpResult := block.NewICmp(enum.IPredNE, ld, constant.NewInt(types.I8, 0))

			lp := loop{
				body: mainFn.NewBlock(""),
				end:  mainFn.NewBlock(""),
			}
			stack = append(stack, lp)

			block.NewCondBr(cmpResult, lp.body, lp.end)
			block = lp.body
		case ']':
			if len(stack) == 0 {
				log.Fatal("unexpected closing bracket")
			}
			lp := stack[len(stack)-1]
			stack = stack[:len(stack)-1]

			ptr := block.NewGetElementPtr(arrayType, cellMem, constant.NewInt(types.I64, 0), block.NewLoad(types.I64, pc))
			ld := block.NewLoad(types.I8, ptr)

			cmpResult := block.NewICmp(enum.IPredNE, ld, constant.NewInt(types.I8, 0))

			block.NewCondBr(cmpResult, lp.body, lp.end)
			block = lp.end
		default:
		}
	}

	if len(stack) != 0 {
		log.Fatal("excessive opening brackets")
	}

	block.NewRet(constant.NewInt(types.I32, 0))

	ll, err := os.CreateTemp(os.TempDir(), "*.ll")
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.WriteString(ll, mod.String())
	if err != nil {
		log.Fatal(err)
	}
	ll.Close()

	cmd := exec.Command("clang", "-w", "-O3", "-o", *output, ll.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}

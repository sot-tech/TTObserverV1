/*
 * BSD-3-Clause
 * Copyright 2020 sot (aka PR_713, C_rho_272)
 * Redistribution and use in source and binary forms, with or without modification,
 * are permitted provided that the following conditions are met:
 * 1. Redistributions of source code must retain the above copyright notice,
 * this list of conditions and the following disclaimer.
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 * this list of conditions and the following disclaimer in the documentation and/or
 * other materials provided with the distribution.
 * 3. Neither the name of the copyright holder nor the names of its contributors
 * may be used to endorse or promote products derived from this software without
 * specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
 * ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
 * WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED.
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT,
 * INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING,
 * BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA,
 * OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY,
 * WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
 * ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY
 * OF SUCH DAMAGE.
 */

package main

import (
	"flag"
	"github.com/op/go-logging"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"sot-te.ch/TTObserverV1"
	"syscall"
)

func main() {
	flag.Parse()
	args := flag.Args()
	logger := logging.MustGetLogger("main")
	if len(args) == 0 {
		logger.Fatal("Observer file not set")
	}
	tt, err := TTObserver.ReadConfig(args[0])
	if err != nil {
		logger.Fatal(err)
	}
	var outputWriter io.Writer
	if tt.Log.File == "" {
		outputWriter = os.Stdout
	} else {
		outputWriter, err = os.OpenFile(tt.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	}
	if err == nil {
		backend := logging.AddModuleLevel(
			logging.NewBackendFormatter(
				logging.NewLogBackend(outputWriter, "", 0),
				logging.MustStringFormatter("%{time:2006-01-02 15:04:05.000}\t%{shortfile}\t%{shortfunc}\t%{level}:\t%{message}")))
		var level logging.Level
		if level, err = logging.LogLevel(tt.Log.Level); err != nil {
			println(err)
			level = logging.INFO
		}
		backend.SetLevel(level, "")
		logging.SetBackend(backend)
	} else {
		println(err)
	}

	if err = tt.Init(); err == nil {
		ch := make(chan os.Signal, 2)
		go func() {
			tt.Engage()
			ch <- syscall.SIGABRT
		}()
		defer tt.Close()
		if len(args) > 1 && len(args[1]) > 0 {
			go func() {
				runtime.GC()
				if err := http.ListenAndServe("localhost:" + args[1], nil); err != nil {
					logger.Error(err)
					ch <- syscall.SIGABRT
				}
			}()
		}
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		if sig == syscall.SIGABRT {
			os.Exit(1)
		}
	} else {
		println(err)
	}
}

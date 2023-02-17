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
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/op/go-logging"

	tto "sot-te.ch/TTObserverV1"
)

var logger = logging.MustGetLogger("main")

func main() {
	configPath := flag.String("c", "/etc/ttobserver/main.json", "Config path")
	m := flag.Bool("m", false, "Migrate from one DB driver to another. Both DB properties must be provided")
	f := flag.String("f", "", "Driver name from what database extract data. Supported values: sqlite3, redis, postgres")
	t := flag.String("t", "", "Driver name to what database import data. Supported values: sqlite3, redis, postgres")
	flag.Parse()
	tt, err := tto.ReadConfig(*configPath)
	if err != nil {
		logger.Fatal(err)
	}
	var outputWriter io.Writer
	if tt.Log.File == "" {
		outputWriter = os.Stdout
	} else {
		outputWriter, err = os.OpenFile(tt.Log.File, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
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
	if *m {
		if len(*f) > 0 && len(*t) > 0 {
			migrate(tt, *f, *t)
		} else {
			println("'f' and 't' keys must be both provided")
			os.Exit(1)
		}
	} else if len(tt.Cluster.NatsURL) > 0 {
		startClustered(tt)
	} else {
		start(tt)
	}
}

func start(tt *tto.Observer) {
	if err := tt.Init(); err == nil {
		ch := make(chan os.Signal, 2)
		go func() {
			tt.Engage()
			ch <- syscall.SIGABRT
		}()
		defer tt.Close()
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		if sig == syscall.SIGABRT {
			os.Exit(1)
		}
	} else {
		logger.Error(err)
	}
}

func startClustered(tt *tto.Observer) {
	tt.Cluster.StartFn = func() (err error) {
		if err = tt.Init(); err == nil {
			tt.Engage()
		}
		return
	}
	tt.Cluster.SuspendFn = tt.Close
	ch := make(chan os.Signal, 2)
	go func() {
		if err := tt.Cluster.Start(); err != nil {
			logger.Error(err)
		}
		ch <- syscall.SIGABRT
	}()
	defer tt.Cluster.Stop()
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	sig := <-ch
	if sig == syscall.SIGABRT {
		os.Exit(1)
	}
}

include $(GOROOT)/src/Make.$(GOARCH)

TARG=rfcd
GOFILES=\
	rfcd.go\


include $(GOROOT)/src/Make.pkg

CLEANFILES+=rfcd

rfcd.$(O): rfcd.go
	$(QUOTED_GOBIN)/$(GC) -I_obj $<

rfcd: rfcd.$(O)
	$(QUOTED_GOBIN)/$(LD) -L_obj -o $@ $<


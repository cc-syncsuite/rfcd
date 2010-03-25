include $(GOROOT)/src/Make.$(GOARCH)

TARG=rfcd
GOFILES=\
	$(TARG).go\


include $(GOROOT)/src/Make.pkg

CLEANFILES+=$(TARG)

$(TARG).$(O): $(TARG).go
	$(QUOTED_GOBIN)/$(GC) -I_obj $<

$(TARG): $(TARG).$(O)
	$(QUOTED_GOBIN)/$(LD) -L_obj -o $@ $<


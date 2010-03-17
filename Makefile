include $(GOROOT)/src/Make.$(GOARCH)

TARG=rfcd
GOFILES=\
	rfcd.go\


include $(GOROOT)/src/Make.pkg

link:
	$(LD) _obj/$(TARG).a
run:
	./$(O).out

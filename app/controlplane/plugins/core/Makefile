# Find plugins subdirectories that have a makefile
SUBDIRS := $(patsubst %Makefile,%,$(wildcard */v1/Makefile))

TARGETS := build

.SECONDEXPANSION:

$(TARGETS): %: subdirs

subdirs: $(SUBDIRS)

$(SUBDIRS):
	$(MAKE) -C $@ $(MAKECMDGOALS)

.PHONY: subdirs $(SUBDIRS) $(TARGETS)

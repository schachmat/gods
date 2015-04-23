#gods

##Summary

„gods“ stands for „Go dwm status“. It displays time, sysload, memory
consumption, battery level and network transfer speeds.

##Dependencies

Only a working Go environment and the xsetroot binary is needed. Per default you
should use my [status font](https://github.com/schachmat/status-18) within dwm,
so you have the nice little icons. Otherwise you need to exchange some
characters in the source (see gods.go header). For dwm the [statuscolor
patch](http://dwm.suckless.org/patches/statuscolors) is recommended.

##Usage

To install, run

	go get github.com/schachmat/gods

Then add the following line to your `.xinitrc` or whereever you start dwm, but
before actually starting dwm:

	$GOPATH/bin/gods &

##Configuration

The Gods status bar can be easily modified, just by patching the source. You can
add new informational panels, remove others, change the ordering or formating.
With a custom font you can use own icons and separators and through the
statuscolors patch config in dwm you can change the colors.

##Contributing

This repository is meant as an example of how to draw your dwm status bar with
go, as I use it. No additional features will be merged from pull-requests, but
you can tell me about your fork and changes and then I can link them here as
further examples for other people to look at.

##License

"THE BEER-WARE LICENSE" (Revision 42):
<teichm@in.tum.de> wrote this file. As long as you retain this notice you
can do whatever you want with this stuff. If we meet some day, and you think
this stuff is worth it, you can buy me a beer in return Markus Teich

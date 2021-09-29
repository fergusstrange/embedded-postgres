module github.com/fergusstrange/embedded-postgres/platform-test

replace github.com/fergusstrange/embedded-postgres => ../

// To avoid CVE CVE-2021-29482
replace github.com/ulikunitz/xz => github.com/ulikunitz/xz v0.5.8

go 1.13

require github.com/fergusstrange/embedded-postgres v0.0.0

//go:build windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	advapi          = syscall.NewLazyDLL("advapi32.dll")
	restrictedToken = advapi.NewProc("CreateRestrictedToken")
	kernel          = syscall.NewLazyDLL("kernel32.dll")
	createJob       = kernel.NewProc("CreateJobObjectW")
	setJob          = kernel.NewProc("SetInformationJobObject")
	assignJob       = kernel.NewProc("AssignProcessToJobObject")
)

func ValidateIdentity(id *Identity) error {
	if id != nil {
		return errors.New("RunAs UID:GID is supported on Unix; Windows uses a restricted token")
	}
	return nil
}

func configure(cmd *exec.Cmd, id *Identity) (func(), error) {
	if err := ValidateIdentity(id); err != nil {
		return nil, err
	}
	current, err := syscall.GetCurrentProcess()
	if err != nil {
		return nil, err
	}
	var original syscall.Token
	if err = syscall.OpenProcessToken(current, syscall.TOKEN_DUPLICATE|syscall.TOKEN_ASSIGN_PRIMARY|syscall.TOKEN_QUERY, &original); err != nil {
		return nil, err
	}
	defer original.Close()
	admin, err := syscall.StringToSid("S-1-5-32-544")
	if err != nil {
		return nil, err
	}
	power, err := syscall.StringToSid("S-1-5-32-547")
	if err != nil {
		return nil, err
	}
	denied := []syscall.SIDAndAttributes{{Sid: admin}, {Sid: power}}
	var token syscall.Token
	ok, _, callErr := restrictedToken.Call(uintptr(original), 1, uintptr(len(denied)), uintptr(unsafe.Pointer(&denied[0])), 0, 0, 0, 0, uintptr(unsafe.Pointer(&token)))
	runtime.KeepAlive(denied)
	if ok == 0 {
		return nil, fmt.Errorf("create restricted token: %w", callErr)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Token: token, CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP, HideWindow: true}
	return func() { _ = token.Close() }, nil
}

func Kill(p *os.Process) error {
	if p == nil {
		return nil
	}
	return p.Kill()
}
func Own(_ string, _ *Identity) error { return nil }

type basicLimits struct {
	PerProcessUserTimeLimit, PerJobUserTimeLimit int64
	LimitFlags                                   uint32
	MinimumWorkingSetSize, MaximumWorkingSetSize uintptr
	ActiveProcessLimit                           uint32
	Affinity                                     uintptr
	PriorityClass, SchedulingClass               uint32
}
type ioCounters struct{ ReadOperationCount, WriteOperationCount, OtherOperationCount, ReadTransferCount, WriteTransferCount, OtherTransferCount uint64 }
type extendedLimits struct {
	Basic                                                                        basicLimits
	IO                                                                           ioCounters
	ProcessMemoryLimit, JobMemoryLimit, PeakProcessMemoryUsed, PeakJobMemoryUsed uintptr
}

// Guard puts the supervisor in a kill-on-close job before it creates children.
// The non-inheritable handle is deliberately retained until process exit. Closing
// it explicitly would also terminate this supervisor before it can report success.
func Guard() error {
	job, _, err := createJob.Call(0, 0)
	if job == 0 {
		return fmt.Errorf("create job: %w", err)
	}
	limits := extendedLimits{}
	limits.Basic.LimitFlags = 0x2000
	ok, _, err := setJob.Call(job, 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		syscall.CloseHandle(syscall.Handle(job))
		return fmt.Errorf("configure job: %w", err)
	}
	current, e := syscall.GetCurrentProcess()
	if e != nil {
		syscall.CloseHandle(syscall.Handle(job))
		return e
	}
	ok, _, err = assignJob.Call(job, uintptr(current))
	if ok == 0 {
		syscall.CloseHandle(syscall.Handle(job))
		return fmt.Errorf("assign job: %w", err)
	}
	return nil
}

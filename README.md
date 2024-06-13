# tofail

Repeat a command *to failure*, i.e., until it returns with non-zero exit code.

This is intended as a helper utility to reproduce sporadic errors. Best used in
combination with sanitizers, AppVerifier, or similar instrumentation.

### Examples

#### Execute the `false` command until it fails, i.e., once:
```console
❯ tofail false
Executing command  [false]  repeatedly in  1  job(s)

>> Failure #1 encountered! Exit code: 1. Output:

<< (#1 output end, exit code 1)
```

#### Execute some program until it fails:
```console
❯ tofail ./my-buggy-program --my-prog-arg
Executing command  [false]  repeatedly in  1  job(s)

>> Failure #1 encountered! Exit code: 1. Output:
Assertion failed in my_superoptimized_kernel.cpp:122: assert(skill > 0);
<< (#1 output end, exit code 1)
```

#### Execute some program until it fails, in multiple parallel jobs:
```console
❯ tofail -j 4 ./my-buggy-program --my-prog-arg
Executing command  [false]  repeatedly in  1  job(s)

>> Failure #1 encountered! Exit code: 1. Output:
Assertion failed in my_superoptimized_kernel.cpp:122: assert(skill > 0);
<< (#1 output end, exit code 1)
Quitting all jobs.

>> Failure #2 encountered! Exit code: 1. Output:
Assertion failed in my_superoptimized_kernel.cpp:122: assert(skill > 0);
<< (#2 output end, exit code 1)
```
`tofail` will stop all jobs when the first failure is encountered, some remaining jobs may
still also fail.

#### Execute a command until it fails *or hits a timeout*:
```console
❯ tofail --timeout 1 sleep 2
Executing command  [sleep 2]  repeatedly in  1  job(s)

Timeout encountered! PID is 97218, process is connected to output file './.tofail_oup2395033973'
The output file will not be deleted automatically.
```
You will get the chance to atach a debugger to the stuck process.

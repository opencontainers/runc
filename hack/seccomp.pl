#!/usr/bin/perl

# ./seccomp.pl < syscall.sample > seccompsyscall.go

use strict;
use warnings;

my $pid = open(my $in, "-|") // die "Couldn't fork1 ($!)\n";

if($pid == 0) {
    $pid = open(my $out, "|-") // die "Couldn't fork2 ($!)\n";
    if($pid == 0) { 
        exec "cpp" or die "Couldn't exec cpp ($!)\n";
        exit 1;
    }
 
    print $out "#include <sys/syscall.h>\n";
    while(<>) {
        if(/^\w/) {
		    my $name="$_";
			chomp($name);

            print $out $name;
			print $out " = ";
			print $out "__NR_$_";
        }
    }
    close $out;
    exit 0;
}
print "//";
system("uname -m");
print "package seccomp\r\n\r\n";
print "var syscallMap = map[string] int {\n";
while(<$in>) {
    my $line=$_;
	
	if($line =~ /^[\da-z_]/)
	{
		my @personal=split(/=/);
		$personal[0] =~ s/[ ]//;
		$personal[1] =~ s/[\r\n]//;
		print "	\"";
		print $personal[0];
		print "\"";
		print " : ";
		if (($personal[1] !~ /[0-9]/) || length($personal[1]) > 4)
		{
			print "-1,\r\n";			
		}else{
			print $personal[1];
			print ",\r\n";
		}
	}
}

print "}\r\n";


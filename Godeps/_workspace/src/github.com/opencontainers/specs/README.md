# Open Container Specifications

This project is where the Open Container Project Specifications are written. This is a work in progress. We should have a first draft by end of July 2015.

Table of Contents

- [Filesystem Bundle](bundle.md)
- [Container Configuration](config.md)
  - [Linux Specific Configuration](config-linux.md)
- [Runtime and Lifecycle](runtime.md)

# The 5 principles of Standard Containers

Define a unit of software delivery called a Standard Container. The goal of a Standard Container is to encapsulate a software component and all its dependencies in a format that is self-describing and portable, so that any compliant runtime can run it without extra dependencies, regardless of the underlying machine and the contents of the container.

The specification for Standard Containers is straightforward. It mostly defines 1) a file format, 2) a set of standard operations, and 3) an execution environment.

A great analogy for this is the shipping container. Just like how Standard Containers are a fundamental unit of software delivery, shipping containers are a fundamental unit of physical delivery.

## 1. Standard operations

Just like shipping containers, Standard Containers define a set of STANDARD OPERATIONS. Shipping containers can be lifted, stacked, locked, loaded, unloaded and labelled. Similarly, Standard Containers can be created, started, and stopped using standard container tools (what this spec is about); copied and snapshotted using standard filesystem tools; and downloaded and uploaded using standard network tools.

## 2. Content-agnostic

Just like shipping containers, Standard Containers are CONTENT-AGNOSTIC: all standard operations have the same effect regardless of the contents. A shipping container will be stacked in exactly the same way whether it contains Vietnamese powder coffee or spare Maserati parts. Similarly, Standard Containers are started or uploaded in the same way whether they contain a postgres database, a php application with its dependencies and application server, or Java build artifacts.

## 3. Infrastructure-agnostic

Both types of containers are INFRASTRUCTURE-AGNOSTIC: they can be transported to thousands of facilities around the world, and manipulated by a wide variety of equipment. A shipping container can be packed in a factory in Ukraine, transported by truck to the nearest routing center, stacked onto a train, loaded into a German boat by an Australian-built crane, stored in a warehouse at a US facility, etc. Similarly, a standard container can be bundled on my laptop, uploaded to S3, downloaded, run and snapshotted by a build server at Equinix in Virginia, uploaded to 10 staging servers in a home-made Openstack cluster, then sent to 30 production instances across 3 EC2 regions.

## 4. Designed for automation

Because they offer the same standard operations regardless of content and infrastructure, Standard Containers, just like their physical counterparts, are extremely well-suited for automation. In fact, you could say automation is their secret weapon.

Many things that once required time-consuming and error-prone human effort can now be programmed. Before shipping containers, a bag of powder coffee was hauled, dragged, dropped, rolled and stacked by 10 different people in 10 different locations by the time it reached its destination. 1 out of 50 disappeared. 1 out of 20 was damaged. The process was slow, inefficient and cost a fortune - and was entirely different depending on the facility and the type of goods.

Similarly, before Standard Containers, by the time a software component ran in production, it had been individually built, configured, bundled, documented, patched, vendored, templated, tweaked and instrumented by 10 different people on 10 different computers. Builds failed, libraries conflicted, mirrors crashed, post-it notes were lost, logs were misplaced, cluster updates were half-broken. The process was slow, inefficient and cost a fortune - and was entirely different depending on the language and infrastructure provider.

## 5. Industrial-grade delivery

There are 17 million shipping containers in existence, packed with every physical good imaginable. Every single one of them can be loaded onto the same boats, by the same cranes, in the same facilities, and sent anywhere in the World with incredible efficiency. It is embarrassing to think that a 30 ton shipment of coffee can safely travel half-way across the World in *less time* than it takes a software team to deliver its code from one datacenter to another sitting 10 miles away.

With Standard Containers we can put an end to that embarrassment, by making INDUSTRIAL-GRADE DELIVERY of software a reality.


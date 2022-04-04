# Future

One wishlist's item is asynchronous rendering: Consider rendering monthly invoices on the first
of each month; depending on the amount of customers/contracts/invoice positions, this can easily
mean you need to render a few thousand PDF documents.

Usually, the PDF generation is not time critical, i.e. they should finish in a reasonable amount of
time (say, within the next 6h to ensure timely delivery to the customer via email). For this to
work, the client could provide a callback URL to which texd sends the PDF via HTTP POST when
the rendering is finished.

Of course, this will also increase complexity on both sides: The client must be network-reachable
itself, an keep track of rendering request in order to associate the PDF to the correct invoice;
texd on the other hand would need a priority queue (processing async documents only if no sync
documents are enqueued), and it would need to store the callback URL somewhere.

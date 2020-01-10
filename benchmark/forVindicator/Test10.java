package test;

public class Test10 extends Thread {
  static int x;
  static int y;
  static int z;
  static final Object m = new Object();
  static final Object n = new Object();
  static final Object o = new Object();
  static final Object p = new Object();
  static final Object q = new Object();

  static void sleepSec(float sec) {
    try {
	Thread.sleep((long)(sec*1000));
    } catch (Exception e) {
       throw new RuntimeException(e);
    }
  }


  @Override
  public void run() {
   	//T1
	synchronized(m) {		
		System.out.println("l(m)");
		synchronized(n) {
			System.out.println("l(n)");
			System.out.println("u(n)");
		}
		int tmp = x;
		System.out.println("r(x)");
		System.out.println("u(m)");
	}
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			synchronized(n) {
				System.out.println("l(n)");
				synchronized(o) {
					System.out.println("l(o)");
					synchronized(p) {
						System.out.println("l(p)");
						y = 1;
						System.out.println("w(y)");
						System.out.println("u(p)");
					}
					System.out.println("u(o)");				
				}
				sleepSec(30);
				synchronized(q) {
					System.out.println("l(q)");
					int tmp = z;
					System.out.println("r(z)");
					System.out.println("u(q)");
				}
				System.out.println("u(n)");
			}
			
		}
   }
   
   public static class T3 extends Thread implements Runnable {
		public void run() {
			sleepSec(10);
			synchronized(o) {
				System.out.println("l(o)");
				synchronized(m) {
					System.out.println("l(m)");
					System.out.println("u(m)");
				}
				synchronized(p) {
					System.out.println("l(p)");
					int tmp = y;
					System.out.println("r(y)");
					System.out.println("u(p)");
				}
				x = 2;
				System.out.println("w(x)");
				System.out.println("u(o)");
			}
			synchronized(q) {
				System.out.println("l(q)");
				z = 1;
				System.out.println("w(z)");
				System.out.println("u(q)");
			}
		}
   }


   public static void main(String args[]) throws Exception {
	final Test10 thread1 = new Test10();
    final T2 thread2 = new T2();
	final T3 thread3 = new T3();

	thread1.start();
	thread2.start();
	thread3.start();

	thread1.join();
	thread2.join();
	thread3.join();

   }
}

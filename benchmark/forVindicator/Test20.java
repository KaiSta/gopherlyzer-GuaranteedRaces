package test;

public class Test20 extends Thread {
  static int x;
  static int a;
  static int y;
  static int z;
  static final Object m = new Object();
  static final Object n = new Object();

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
		x = 1;
		System.out.println("w(x)");
		y = 1;
		System.out.println("w(y)");
		System.out.println("u(m)");
	}
  } 

   public static class T2 extends Thread implements Runnable {
		public void run() {
			sleepSec(5);
			synchronized(m) {
				System.out.println("l(m)");
				synchronized(n) {
					System.out.println("l(n)");
					z = 1;
					System.out.println("w(z)");
					System.out.println("u(n)");
				}
				sleepSec(30);
				synchronized(n) {
					System.out.println("l(n)");
					int tmp = z;
					System.out.println("r(z)");
					System.out.println("u(n)");
				}
				int tmp =  y;
				System.out.println("r(y)");
				System.out.println("u(m)");
			}
		}
   }

   public static class T3 extends Thread implements Runnable {
		public void run() {
			sleepSec(10);
			synchronized(n) {
				System.out.println("l(n)");
				int tmp = z;
				z = 2;
				System.out.println("r(z)");
				System.out.println("u(n)");
			}
			x = 2;
			System.out.println("w(x)");
		}
   }

   public static void main(String args[]) throws Exception {
	final Test20 thread1 = new Test20();
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
